// Copyright 2015 CoreOS, Inc.
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

#include <stdint.h>
#include <sys/capability.h>
#include <sys/prctl.h>

#include "fatal.h"

/* drop capabilities */
void capabilities_drop(uint64_t todrop)
{
#define cap(_sym, _desc)					\
	if((todrop & (1ULL << _sym))) {				\
		pexit_if(prctl(PR_CAPBSET_DROP, _sym) == -1,	\
			 "error dropping " #_sym " capability");\
	}
#define cap_keep(_sym, _desc) cap(_sym, _desc)
#include "defs/capabilities.def"
}
