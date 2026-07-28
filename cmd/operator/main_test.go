/*
Copyright 2026 The KEDA Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	kedautil "github.com/kedacore/keda/v2/pkg/util"
)

func TestResolveHTTPDisableKeepAlive(t *testing.T) {
	testCases := []struct {
		name          string
		envValue      *string
		flagValue     bool
		flagChanged   bool
		expectedValue bool
		expectedUsed  bool
	}{
		{name: "defaults to false"},
		{name: "explicit flag", flagValue: true, flagChanged: true, expectedValue: true},
		{name: "legacy environment fallback", envValue: stringPtr("true"), expectedValue: true, expectedUsed: true},
		{name: "explicit false flag overrides environment", envValue: stringPtr("true"), flagChanged: true, expectedUsed: true},
		{name: "invalid legacy value preserves disabled behavior", envValue: stringPtr("invalid"), expectedUsed: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			originalValue, originallySet := os.LookupEnv(kedautil.HTTPDisableKeepAliveEnvVar)
			t.Cleanup(func() {
				if originallySet {
					assert.NoError(t, os.Setenv(kedautil.HTTPDisableKeepAliveEnvVar, originalValue))
				} else {
					assert.NoError(t, os.Unsetenv(kedautil.HTTPDisableKeepAliveEnvVar))
				}
			})

			if testCase.envValue == nil {
				assert.NoError(t, os.Unsetenv(kedautil.HTTPDisableKeepAliveEnvVar))
			} else {
				assert.NoError(t, os.Setenv(kedautil.HTTPDisableKeepAliveEnvVar, *testCase.envValue))
			}

			value, used := resolveHTTPDisableKeepAlive(testCase.flagValue, testCase.flagChanged)
			assert.Equal(t, testCase.expectedValue, value)
			assert.Equal(t, testCase.expectedUsed, used)
		})
	}
}

func stringPtr(value string) *string {
	return &value
}
