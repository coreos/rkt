// Copyright 2016 CNI authors
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

package skel

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Skel", func() {
	var (
		fNoop = func(_ *CmdArgs) error { return nil }
		// fErr    = func(_ *CmdArgs) error { return errors.New("dummy") }
		envVars = []struct {
			name string
			val  string
		}{
			{"CNI_CONTAINERID", "dummy"},
			{"CNI_NETNS", "dummy"},
			{"CNI_IFNAME", "dummy"},
			{"CNI_ARGS", "dummy"},
			{"CNI_PATH", "dummy"},
		}
	)

	It("Must be possible to set the env vars", func() {
		for _, v := range envVars {
			err := os.Setenv(v.name, v.val)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("When dummy environment variables are passed", func() {

		It("should not fail with ADD and noop callback", func() {
			err := os.Setenv("CNI_COMMAND", "ADD")
			Expect(err).NotTo(HaveOccurred())
			PluginMain(fNoop, nil)
		})

		// TODO: figure out howto mock printing and os.Exit()
		// It("should fail with ADD and error callback", func() {
		// 	err := os.Setenv("CNI_COMMAND", "ADD")
		// 	Expect(err).NotTo(HaveOccurred())
		// 	PluginMain(fErr, nil)
		// })

		It("should not fail with DEL and noop callback", func() {
			err := os.Setenv("CNI_COMMAND", "DEL")
			Expect(err).NotTo(HaveOccurred())
			PluginMain(nil, fNoop)
		})

		// TODO: figure out howto mock printing and os.Exit()
		// It("should fail with DEL and error callback", func() {
		// 	err := os.Setenv("CNI_COMMAND", "DEL")
		// 	Expect(err).NotTo(HaveOccurred())
		// 	PluginMain(fErr, nil)
		// })

		It("should not fail with DEL and no NETNS and noop callback", func() {
			err := os.Setenv("CNI_COMMAND", "DEL")
			Expect(err).NotTo(HaveOccurred())
			err = os.Unsetenv("CNI_NETNS")
			Expect(err).NotTo(HaveOccurred())
			PluginMain(nil, fNoop)
		})

	})
})
