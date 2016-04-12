// Copyright 2015 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/rkt/tests/testutils"
	taas "github.com/coreos/rkt/tests/testutils/aci-server"
)

func runImage(t *testing.T, ctx *testutils.RktRunCtx, imageFile string, expected string, shouldFail bool) {
	cmd := fmt.Sprintf(`%s --debug run --mds-register=false %s`, ctx.Cmd(), imageFile)
	runRktAndCheckOutput(t, cmd, expected, shouldFail)
}

func TestTrust(t *testing.T) {
	imageFile := patchTestACI("rkt-inspect-trust1.aci", "--exec=/inspect --print-msg=Hello", "--name=rkt-prefix.com/my-app")
	defer os.Remove(imageFile)

	imageFile2 := patchTestACI("rkt-inspect-trust2.aci", "--exec=/inspect --print-msg=Hello", "--name=rkt-alternative.com/my-app")
	defer os.Remove(imageFile2)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()

	t.Logf("Run the unsigned image: it should fail")
	runImage(t, ctx, imageFile, "error opening signature file", true)

	t.Logf("Sign the images")
	ascFile := runSignImage(t, imageFile, 1)
	defer os.Remove(ascFile)
	ascFile = runSignImage(t, imageFile2, 1)
	defer os.Remove(ascFile)

	t.Logf("Run the signed image without trusting the key: it should fail")
	runImage(t, ctx, imageFile, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key with the wrong prefix")
	runRktTrust(t, ctx, "wrong-prefix.com/my-app", 1)

	t.Logf("Run a signed image with the key installed in the wrong prefix: it should fail")
	runImage(t, ctx, imageFile, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key with the correct prefix, but wrong key")
	runRktTrust(t, ctx, "rkt-prefix.com/my-app", 2)

	t.Logf("Run a signed image with the wrong key installed: it should fail")
	runImage(t, ctx, imageFile, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key with the correct prefix")
	runRktTrust(t, ctx, "rkt-prefix.com/my-app", 1)

	t.Logf("Finally, run successfully the signed image")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key on unrelated prefixes")
	runRktTrust(t, ctx, "foo.com", 1)
	runRktTrust(t, ctx, "example.com/my-app", 1)

	t.Logf("But still only the first image can be executed")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "openpgp: signature made by unknown entity", true)

	t.Logf("Trust the key for all images (rkt trust --root)")
	runRktTrust(t, ctx, "", 1)

	t.Logf("Now both images can be executed")
	runImage(t, ctx, imageFile, "Hello", false)
	runImage(t, ctx, imageFile2, "Hello", false)
}

func TestTrustDiscovery(t *testing.T) {
	const (
		goodKeyIdx   = 1
		wrongKeyIdx  = 2
		stage1KeyIdx = 3
	)

	ctx := testutils.NewRktRunCtx()
	defer ctx.Cleanup()
	server := runServer(t, taas.GetDefaultServerSetup())
	defer server.Close()
	stubStage1 := testutils.GetValueFromEnvOrPanic("STUB_STAGE1")
	ourStubStage1 := filepath.Join(getFunctionalTmpDir(), filepath.Base(stubStage1))
	baseName := "rkt-inspect-discovery-trust"
	imageName := fmt.Sprintf("localhost/%s", baseName)
	// create an image
	imageFile := patchTestACI(fmt.Sprintf("%s.aci", baseName), fmt.Sprintf("--name=%s", imageName))
	defer os.Remove(imageFile)
	// create a good signature
	ascGoodKeyGoodImage := runSignImage(t, imageFile, goodKeyIdx)
	defer os.Remove(ascGoodKeyGoodImage)
	// create a wrong signature - wrong image signed with a good key
	ascGoodKeyWrongImage := filepath.Join(getFunctionalTmpDir(), "rkt-trust-discovery-wrong-image.asc")
	runSignImageToFile(t, getEmptyImagePath(), ascGoodKeyWrongImage, goodKeyIdx)
	defer os.Remove(ascGoodKeyWrongImage)
	// create a wrong signature - wrong image signed with a wrong key
	ascWrongKeyWrongImage := filepath.Join(getFunctionalTmpDir(), "rkt-trust-discovery-wrong-key.asc")
	runSignImageToFile(t, getEmptyImagePath(), ascWrongKeyWrongImage, wrongKeyIdx)
	defer os.Remove(ascWrongKeyWrongImage)
	// we are not generating ascWrongKeyGoodImage, because testing
	// it together with the wrong key is the same as testing
	// ascGoodKeyGoodImage with the good key

	// we are going to use stub stage1 image via --stage1-path
	// flag and we will not pass --insecure-options=image, so we
	// need to sign the image. since the signature is being looked
	// for in the same directory as stage1 we need to create a
	// symlink to the image in the directory we can write
	if err := os.Symlink(stubStage1, ourStubStage1); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(ourStubStage1)
	ourStubStage1Signature := runSignImage(t, stubStage1, stage1KeyIdx)
	defer os.Remove(ourStubStage1Signature)

	type HTTPKeyQuestion int
	const (
		HTTPSKeyNoQuestionExpected HTTPKeyQuestion = iota
		HTTPSKeyQuestionExpectedReject
		HTTPSKeyQuestionExpectedAccept
	)

	type testCase struct {
		name           string
		uploads        map[string]string
		trustHTTPSKeys bool
		HTTPSKeyQuery  HTTPKeyQuestion
		exitStatus     int
		trustedKey     *gpgkey
		expected       []string
	}

	const (
		signatureMissing = iota
		signatureWrongImage
		signatureWrongKey
		signatureGood

		signatureRange
	)

	const (
		serverKeyMissing = iota
		serverKeyWrong
		serverKeyGood

		serverKeyRange
	)

	const (
		trustedKeyMissing = iota
		trustedKeyWrong
		trustedKeyGood

		trustedKeyRange
	)

	const (
		trustHTTPSKeyReject = iota
		trustHTTPSKeyAccept
		trustHTTPSKeyTrust

		trustHTTPSKeyRange
	)

	possibilities := signatureRange * serverKeyRange * trustedKeyRange * trustHTTPSKeyRange
	tests := make([]*testCase, 0, possibilities)
	for i := 0; i < possibilities; i++ {
		value := i
		signatureOpt := value % signatureRange
		value = value / signatureRange
		serverKeyOpt := value % serverKeyRange
		value = value / serverKeyRange
		trustedKeyOpt := value % trustedKeyRange
		value = value / trustedKeyRange
		trustHTTPSKeyOpt := value % trustHTTPSKeyRange
		value = value / trustHTTPSKeyRange
		// this will hold parts of the test name based on the
		// above four *Opt variables and an expected result
		nameParts := make([]string, 5)
		uploads := map[string]string{
			filepath.Base(imageFile): imageFile,
		}
		var trustedKey *gpgkey
		exitStatus := 1
		var expected []string
		// by default, no review key is expected, either we
		// will use trusted key or we pass
		// --trust-keys-from-https
		HTTPSKeyQuery := HTTPSKeyNoQuestionExpected
		switch signatureOpt {
		case signatureMissing:
			nameParts[0] = "no signature"
		case signatureWrongImage:
			nameParts[0] = "wrong signature (signed with good key)"
			uploads[filepath.Base(ascGoodKeyGoodImage)] = ascGoodKeyWrongImage
		case signatureWrongKey:
			nameParts[0] = "wrong signature (signed with wrong key)"
			uploads[filepath.Base(ascGoodKeyGoodImage)] = ascWrongKeyWrongImage
		case signatureGood:
			nameParts[0] = "good signature"
			uploads[filepath.Base(ascGoodKeyGoodImage)] = ascGoodKeyGoodImage
		}
		switch serverKeyOpt {
		case serverKeyMissing:
			nameParts[1] = "no key on server"
		case serverKeyWrong:
			nameParts[1] = "wrong key on server"
			uploads[taas.Pubkeys] = getGPGKey(t, wrongKeyIdx).path
		case serverKeyGood:
			nameParts[1] = "good key on server"
			uploads[taas.Pubkeys] = getGPGKey(t, goodKeyIdx).path
		}
		switch trustedKeyOpt {
		case trustedKeyMissing:
			nameParts[2] = "no trusted key"
		case trustedKeyWrong:
			nameParts[2] = "wrong trusted key"
			trustedKey = getGPGKey(t, wrongKeyIdx)
		case trustedKeyGood:
			nameParts[2] = "good trusted key"
			trustedKey = getGPGKey(t, goodKeyIdx)
		}
		switch trustHTTPSKeyOpt {
		case trustHTTPSKeyReject:
			nameParts[3] = "will reject the key in a review"
		case trustHTTPSKeyAccept:
			nameParts[3] = "will accept the key in a review"
		case trustHTTPSKeyTrust:
			nameParts[3] = "will trust the key without a review"
		}
		signatureAvailable := signatureOpt != signatureMissing
		goodSignature := signatureOpt == signatureGood
		willUseTrustedKey := trustedKeyOpt != trustedKeyMissing
		willUseGoodTrustedKey := trustedKeyOpt == trustedKeyGood
		serverKeyAvailable := serverKeyOpt != serverKeyMissing
		goodServerKeyAvailable := serverKeyOpt == serverKeyGood
		mightUseServerKey := trustHTTPSKeyOpt != trustHTTPSKeyReject
		willUseServerKey := !willUseTrustedKey && serverKeyAvailable && mightUseServerKey
		willUseAnyKey := willUseTrustedKey || willUseServerKey
		goodKey := willUseGoodTrustedKey || (!willUseTrustedKey && goodServerKeyAvailable && mightUseServerKey)
		willUseBadKey := willUseAnyKey && !goodKey
		wrongSignatureMatchesKey := (signatureOpt == signatureWrongKey && willUseBadKey) || (signatureOpt == signatureWrongImage && goodKey)
		switch {
		case !signatureAvailable:
			// short circuiting here, following cases in
			// the switch assume that signature is
			// available
		case willUseTrustedKey:
			expected = append(expected, fmt.Sprintf("keys already exist for prefix %q, not fetching again", imageName))
		case !serverKeyAvailable:
			// short circuiting here, following cases in
			// the switch assume that the server has the
			// key
		case trustHTTPSKeyOpt == trustHTTPSKeyReject:
			HTTPSKeyQuery = HTTPSKeyQuestionExpectedReject
		case trustHTTPSKeyOpt == trustHTTPSKeyAccept:
			HTTPSKeyQuery = HTTPSKeyQuestionExpectedAccept
		}
		doAppendKeystoreMessages := true
		switch {
		case !signatureAvailable:
			nameParts[4] = "should fail because there is no signature"
			expected = append(expected, "error downloading the signature file", "bad HTTP status code: 404")
			doAppendKeystoreMessages = false
		case !willUseAnyKey:
			nameParts[4] = "should fail because there is no trusted key"
		case !goodKey && !goodSignature:
			nameParts[4] = "should fail because both trusted key and signature are wrong"
		case !goodKey && goodSignature:
			nameParts[4] = "should fail because the trusted key is wrong"
		case goodKey && !goodSignature:
			nameParts[4] = "should fail because the signature is wrong"
		case goodKey && goodSignature:
			nameParts[4] = "should succeed"
			expected = append(expected, "success, stub stage1 would at this point switch to stage2")
			exitStatus = 0
			doAppendKeystoreMessages = false
		}
		if doAppendKeystoreMessages {
			unknownEntity := "openpgp: signature made by unknown entity"
			doesNotMatch := "openpgp: invalid signature: hash tag doesn't match"
			pgpMessage := ""
			if wrongSignatureMatchesKey {
				// if the signature matches the key,
				// then the initial signature check
				// (happening before the image is
				// downloaded) will pass, but the
				// image validation will fail with a
				// hash tag mismatch error
				pgpMessage = doesNotMatch
			} else {
				// if the signature does not match the
				// key (or the key is missing), then
				// the initial signature check will
				// fail with an unknown entity error
				pgpMessage = unknownEntity
			}
			expected = append(expected, pgpMessage)
		}
		tt := &testCase{
			name:           strings.Join(nameParts, ", "),
			uploads:        uploads,
			trustHTTPSKeys: trustHTTPSKeyOpt == trustHTTPSKeyTrust,
			HTTPSKeyQuery:  HTTPSKeyQuery,
			exitStatus:     exitStatus,
			trustedKey:     trustedKey,
			expected:       expected,
		}
		tests = append(tests, tt)
	}

	for idx, tt := range tests {
		t.Logf("Test #%d: %s", idx, tt.name)
		t.Logf("Trusting stage1 image signing key")
		runRktTrust(t, ctx, "localhost/rkt-stub-stage1", stage1KeyIdx)
		t.Logf("Registering fileset in the server: %#v", tt.uploads)
		if err := server.UpdateFileSet(tt.uploads); err != nil {
			t.Fatal(err)
		}
		if tt.trustedKey != nil {
			t.Logf("Trusting key %q with fingerprint %s", tt.trustedKey.path, tt.trustedKey.fingerprint)
			runRktTrustKey(t, ctx, imageName, tt.trustedKey)
		} else {
			t.Log("No key to trust")
		}
		// passing tls to avoid invalid cert errors and pubkey
		// to actually use the keys from server with an
		// invalid cert
		cmd := fmt.Sprintf(`%s --debug --insecure-options=tls,pubkey --trust-keys-from-https=%v run --stage1-path=%q %s`, ctx.Cmd(), tt.trustHTTPSKeys, stubStage1, imageName)
		child := spawnOrFail(t, cmd)
		insecureKeysMsg := "signing keys may be downloaded from an insecure connection"
		willAccept := true
		switch tt.HTTPSKeyQuery {
		case HTTPSKeyNoQuestionExpected:
		case HTTPSKeyQuestionExpectedReject:
			willAccept = false
			fallthrough
		case HTTPSKeyQuestionExpectedAccept:
			t.Logf("expecting: %s", insecureKeysMsg)
			if err := expectWithOutput(child, insecureKeysMsg); err != nil {
				t.Fatalf("Expected but didn't find %q in %v", insecureKeysMsg, err)
			}
			runGPGKeyReview(t, child, imageName, willAccept)
		}
		for _, expected := range tt.expected {
			t.Logf("expecting: %s", expected)
			if err := expectWithOutput(child, expected); err != nil {
				t.Fatalf("Expected but didn't find %q in %v", expected, err)
			}
		}
		waitOrFail(t, child, tt.exitStatus)
		ctx.Reset()
	}
}
