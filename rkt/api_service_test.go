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
	"testing"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/api/v1alpha"
)

func TestFilterPod(t *testing.T) {
	tests := []struct {
		pod      *v1alpha.Pod
		manifest *schema.PodManifest
		filter   *v1alpha.PodFilter
		filtered bool
	}{
		// Has the status.
		{
			&v1alpha.Pod{
				State: v1alpha.PodState_POD_STATE_RUNNING,
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				States: []v1alpha.PodState{v1alpha.PodState_POD_STATE_RUNNING},
			},
			false,
		},
		// Doesn't have the status.
		{
			&v1alpha.Pod{
				State: v1alpha.PodState_POD_STATE_EXITED,
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				States: []v1alpha.PodState{v1alpha.PodState_POD_STATE_RUNNING},
			},
			true,
		},
		// Has the app name.
		{
			&v1alpha.Pod{
				Apps: []*v1alpha.App{{Name: "app-foo"}},
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				AppNames: []string{"app-first", "app-foo", "app-last"},
			},
			false,
		},
		// Doesn't have the app name.
		{
			&v1alpha.Pod{
				Apps: []*v1alpha.App{{Name: "app-foo"}},
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				AppNames: []string{"app-first", "app-second", "app-last"},
			},
			true,
		},
		// Has the network name.
		{
			&v1alpha.Pod{
				Networks: []*v1alpha.Network{{Name: "network-foo"}},
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				NetworkNames: []string{"network-first", "network-foo", "network-last"},
			},
			false,
		},
		// Doesn't have the network name.
		{
			&v1alpha.Pod{
				Networks: []*v1alpha.Network{{Name: "network-foo"}},
			},
			&schema.PodManifest{},
			&v1alpha.PodFilter{
				NetworkNames: []string{"network-first", "network-second", "network-last"},
			},
			true,
		},
		// Has the annotation.
		{
			&v1alpha.Pod{},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-foo", "annotation-value-foo"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			false,
		},
		// Doesn't have the annotation key.
		{
			&v1alpha.Pod{},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-second", "annotation-value-foo"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			true,
		},
		// Doesn't have the annotation value.
		{
			&v1alpha.Pod{},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-foo", "annotation-value-second"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			true,
		},
		// Doesn't satisfy any filter conditions.
		{
			&v1alpha.Pod{
				Apps:     []*v1alpha.App{{Name: "app-foo"}},
				Networks: []*v1alpha.Network{{Name: "network-foo"}},
			},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				AppNames:     []string{"app-bar"},
				NetworkNames: []string{"network-bar"},
				Annotations:  []*v1alpha.KeyValue{{"annotation-key-foo", "annotation-value-bar"}},
			},
			true,
		},
		// Satisfies some filter conditions.
		{
			&v1alpha.Pod{
				Apps:     []*v1alpha.App{{Name: "app-foo"}},
				Networks: []*v1alpha.Network{{Name: "network-foo"}},
			},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				AppNames:     []string{"app-foo", "app-bar"},
				NetworkNames: []string{"network-bar"},
				Annotations:  []*v1alpha.KeyValue{{"annotation-key-bar", "annotation-value-bar"}},
			},
			true,
		},
		// Satisfies all filter conditions.
		{
			&v1alpha.Pod{
				Apps:     []*v1alpha.App{{Name: "app-foo"}},
				Networks: []*v1alpha.Network{{Name: "network-foo"}},
			},
			&schema.PodManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.PodFilter{
				AppNames:     []string{"app-foo", "app-bar"},
				NetworkNames: []string{"network-bar", "network-foo"},
				Annotations:  []*v1alpha.KeyValue{{"annotation-key-bar", "annotation-value-bar"}, {"annotation-key-foo", "annotation-value-foo"}},
			},
			false,
		},
	}

	for i, tt := range tests {
		result := filterPod(tt.pod, tt.manifest, tt.filter)
		if result != tt.filtered {
			t.Errorf("#%d: got %v, want %v", i, result, tt.filtered)
		}
	}
}

func TestFilterImage(t *testing.T) {
	tests := []struct {
		image    *v1alpha.Image
		manifest *schema.ImageManifest
		filter   *v1alpha.ImageFilter
		filtered bool
	}{
		// Has the id.
		{
			&v1alpha.Image{
				Id: "id-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Ids: []string{"id-first", "id-foo", "id-last"},
			},
			false,
		},
		// Doesn't have the id.
		{
			&v1alpha.Image{
				Id: "id-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Ids: []string{"id-first", "id-second", "id-last"},
			},
			true,
		},
		// Has the prefix in the name.
		{
			&v1alpha.Image{
				Name: "prefix-foo-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Prefixes: []string{"prefix-first", "prefix-foo", "prefix-last"},
			},
			false,
		},
		// Doesn't have the prefix in the name.
		{
			&v1alpha.Image{
				Name: "prefix-foo-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Prefixes: []string{"prefix-first", "prefix-second", "prefix-last"},
			},
			true,
		},
		// Has the base name in the name.
		{
			&v1alpha.Image{
				Name: "foo/basename-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				BaseNames: []string{"basename-first", "basename-foo", "basename-last"},
			},
			false,
		},
		// Doesn't have the base name in the name.
		{
			&v1alpha.Image{
				Name: "foo/basename-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				BaseNames: []string{"basename-first", "basename-second", "basename-last"},
			},
			true,
		},
		// Has the keyword in the name.
		{
			&v1alpha.Image{
				Name: "foo-keyword-foo-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Keywords: []string{"keyword-first", "keyword-foo", "keyword-last"},
			},
			false,
		},
		// Doesn't have the keyword in the name.
		{
			&v1alpha.Image{
				Name: "foo-keyword-foo-foo",
			},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				Keywords: []string{"keyword-first", "keyword-second", "keyword-last"},
			},
			true,
		},
		// Has the label in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Labels: []types.Label{{"label-key-foo", "label-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Labels: []*v1alpha.KeyValue{
					{"label-key-first", "label-value-first"},
					{"label-key-foo", "label-value-foo"},
					{"label-key-last", "label-value-last"},
				},
			},
			false,
		},
		// Doesn't have the label key in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Labels: []types.Label{{"label-key-foo", "label-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Labels: []*v1alpha.KeyValue{
					{"label-key-first", "label-value-first"},
					{"label-key-second", "label-value-foo"},
					{"label-key-last", "label-value-last"},
				},
			},
			true,
		},
		// Doesn't have the label value in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Labels: []types.Label{{"label-key-foo", "label-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Labels: []*v1alpha.KeyValue{
					{"label-key-first", "label-value-first"},
					{"label-key-foo", "label-value-second"},
					{"label-key-last", "label-value-last"},
				},
			},
			true,
		},
		// Has the annotation in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-foo", "annotation-value-foo"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			false,
		},
		// Doesn't have the annotation key in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-second", "annotation-value-foo"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			true,
		},
		// Doesn't have the annotation value in the manifest.
		{
			&v1alpha.Image{},
			&schema.ImageManifest{
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Annotations: []*v1alpha.KeyValue{
					{"annotation-key-first", "annotation-value-first"},
					{"annotation-key-foo", "annotation-value-second"},
					{"annotation-key-last", "annotation-value-last"},
				},
			},
			true,
		},
		// Satisfies 'imported after'.
		{
			&v1alpha.Image{ImportTimestamp: 1024},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				ImportedAfter: 1023,
			},
			false,
		},
		// Doesn't satisfy 'imported after'.
		{
			&v1alpha.Image{ImportTimestamp: 1024},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				ImportedAfter: 1024,
			},
			true,
		},
		// Satisfies 'imported before'.
		{
			&v1alpha.Image{ImportTimestamp: 1024},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				ImportedBefore: 1025,
			},
			false,
		},
		// Doesn't satisfy 'imported before'.
		{
			&v1alpha.Image{ImportTimestamp: 1024},
			&schema.ImageManifest{},
			&v1alpha.ImageFilter{
				ImportedBefore: 1024,
			},
			true,
		},
		// Doesn't satisfy any filter conditions.
		{
			&v1alpha.Image{
				Id:              "id-foo",
				Name:            "prefix-foo-keyword-foo/basename-foo",
				Version:         "1.0",
				ImportTimestamp: 1024,
			},
			&schema.ImageManifest{
				Labels:      []types.Label{{"label-key-foo", "label-value-foo"}},
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Ids:            []string{"id-bar"},
				Prefixes:       []string{"prefix-bar"},
				BaseNames:      []string{"basename-bar"},
				Keywords:       []string{"keyword-bar"},
				Labels:         []*v1alpha.KeyValue{{"label-key-bar", "label-value-bar"}},
				Annotations:    []*v1alpha.KeyValue{{"annotation-key-bar", "annotation-value-bar"}},
				ImportedBefore: 1024,
				ImportedAfter:  1024,
			},
			true,
		},
		// Satisfies some filter conditions.
		{
			&v1alpha.Image{
				Id:              "id-foo",
				Name:            "prefix-foo-keyword-foo/basename-foo",
				Version:         "1.0",
				ImportTimestamp: 1024,
			},
			&schema.ImageManifest{
				Labels:      []types.Label{{"label-key-foo", "label-value-foo"}},
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Ids:            []string{"id-bar", "id-foo"},
				Prefixes:       []string{"prefix-bar"},
				BaseNames:      []string{"basename-bar"},
				Keywords:       []string{"keyword-bar"},
				Labels:         []*v1alpha.KeyValue{{"label-key-bar", "label-value-bar"}},
				Annotations:    []*v1alpha.KeyValue{{"annotation-key-bar", "annotation-value-bar"}},
				ImportedBefore: 1024,
				ImportedAfter:  1024,
			},
			true,
		},
		// Satisfies all filter conditions.
		{
			&v1alpha.Image{
				Id:              "id-foo",
				Name:            "prefix-foo-keyword-foo/basename-foo",
				Version:         "1.0",
				ImportTimestamp: 1024,
			},
			&schema.ImageManifest{
				Labels:      []types.Label{{"label-key-foo", "label-value-foo"}},
				Annotations: []types.Annotation{{"annotation-key-foo", "annotation-value-foo"}},
			},
			&v1alpha.ImageFilter{
				Ids:            []string{"id-bar", "id-foo"},
				Prefixes:       []string{"prefix-bar", "prefix-foo"},
				BaseNames:      []string{"basename-bar", "basename-foo"},
				Keywords:       []string{"keyword-bar", "keyword-foo"},
				Labels:         []*v1alpha.KeyValue{{"label-key-bar", "label-value-bar"}, {"label-key-foo", "label-value-foo"}},
				Annotations:    []*v1alpha.KeyValue{{"annotation-key-bar", "annotation-value-bar"}, {"annotation-key-foo", "annotation-value-foo"}},
				ImportedBefore: 1025,
				ImportedAfter:  1023,
			},
			false,
		},
	}

	for i, tt := range tests {
		result := filterImage(tt.image, tt.manifest, tt.filter)
		if result != tt.filtered {
			t.Errorf("#%d: got %v, want %v", i, result, tt.filtered)
		}
	}
}
