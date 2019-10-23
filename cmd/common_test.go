// Copyright © 2017-2019 Aqua Security Software Ltd. <info@aquasec.com>
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

package cmd

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/aquasecurity/kube-bench/check"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewRunFilter(t *testing.T) {

	type TestCase struct {
		Name       string
		FilterOpts FilterOpts
		Group      *check.Group
		Check      *check.Check

		Expected bool
	}

	testCases := []TestCase{
		{
			Name:       "Should return true when scored flag is enabled and check is scored",
			FilterOpts: FilterOpts{Scored: true, Unscored: false},
			Group:      &check.Group{},
			Check:      &check.Check{Scored: true},
			Expected:   true,
		},
		{
			Name:       "Should return false when scored flag is enabled and check is not scored",
			FilterOpts: FilterOpts{Scored: true, Unscored: false},
			Group:      &check.Group{},
			Check:      &check.Check{Scored: false},
			Expected:   false,
		},

		{
			Name:       "Should return true when unscored flag is enabled and check is not scored",
			FilterOpts: FilterOpts{Scored: false, Unscored: true},
			Group:      &check.Group{},
			Check:      &check.Check{Scored: false},
			Expected:   true,
		},
		{
			Name:       "Should return false when unscored flag is enabled and check is scored",
			FilterOpts: FilterOpts{Scored: false, Unscored: true},
			Group:      &check.Group{},
			Check:      &check.Check{Scored: true},
			Expected:   false,
		},

		{
			Name:       "Should return true when group flag contains group's ID",
			FilterOpts: FilterOpts{Scored: true, Unscored: true, GroupList: "G1,G2,G3"},
			Group:      &check.Group{ID: "G2"},
			Check:      &check.Check{},
			Expected:   true,
		},
		{
			Name:       "Should return false when group flag doesn't contain group's ID",
			FilterOpts: FilterOpts{GroupList: "G1,G3"},
			Group:      &check.Group{ID: "G2"},
			Check:      &check.Check{},
			Expected:   false,
		},

		{
			Name:       "Should return true when check flag contains check's ID",
			FilterOpts: FilterOpts{Scored: true, Unscored: true, CheckList: "C1,C2,C3"},
			Group:      &check.Group{},
			Check:      &check.Check{ID: "C2"},
			Expected:   true,
		},
		{
			Name:       "Should return false when check flag doesn't contain check's ID",
			FilterOpts: FilterOpts{CheckList: "C1,C3"},
			Group:      &check.Group{},
			Check:      &check.Check{ID: "C2"},
			Expected:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			filter, _ := NewRunFilter(testCase.FilterOpts)
			assert.Equal(t, testCase.Expected, filter(testCase.Group, testCase.Check))
		})
	}

	t.Run("Should return error when both group and check flags are used", func(t *testing.T) {
		// given
		opts := FilterOpts{GroupList: "G1", CheckList: "C1"}
		// when
		_, err := NewRunFilter(opts)
		// then
		assert.EqualError(t, err, "group option and check option can't be used together")
	})

}

func TestIsMaster(t *testing.T) {
	testCases := []struct {
		name            string
		cfgFile         string
		getBinariesFunc func(*viper.Viper) (map[string]string, error)
		isMaster        bool
	}{
		{
			name:    "valid config, is master and all components are running",
			cfgFile: "../cfg/config.yaml",
			getBinariesFunc: func(viper *viper.Viper) (strings map[string]string, i error) {
				return map[string]string{"apiserver": "kube-apiserver"}, nil
			},
			isMaster: true,
		},
		{
			name:    "valid config, is master and but not all components are running",
			cfgFile: "../cfg/config.yaml",
			getBinariesFunc: func(viper *viper.Viper) (strings map[string]string, i error) {
				return map[string]string{}, nil
			},
			isMaster: false,
		},
		{
			name:    "valid config, is master, not all components are running and fails to find all binaries",
			cfgFile: "../cfg/config.yaml",
			getBinariesFunc: func(viper *viper.Viper) (strings map[string]string, i error) {
				return map[string]string{}, errors.New("failed to find binaries")
			},
			isMaster: false,
		},
		{
			name:     "valid config, does not include master",
			cfgFile:  "../cfg/node_only.yaml",
			isMaster: false,
		},
	}

	for _, tc := range testCases {
		cfgFile = tc.cfgFile
		initConfig()

		oldGetBinariesFunc := getBinariesFunc
		getBinariesFunc = tc.getBinariesFunc
		defer func() {
			getBinariesFunc = oldGetBinariesFunc
			cfgFile = ""
		}()

		assert.Equal(t, tc.isMaster, isMaster(), tc.name)
	}
}

func TestMapToCISVersion(t *testing.T) {

	viperWithData, err := loadConfigForTest()
	if err != nil {
		t.Fatalf("Unable to load config file %v", err)
	}
	kubeToCISMap, err := loadVersionMapping(viperWithData)
	if err != nil {
		t.Fatalf("Unable to load config file %v", err)
	}

	cases := []struct {
		kubeVersion string
		succeed     bool
		exp         string
	}{
		{kubeVersion: "1.11", succeed: true, exp: "cis-1.3"},
		{kubeVersion: "1.12", succeed: true, exp: "cis-1.3"},
		{kubeVersion: "1.13", succeed: true, exp: "cis-1.4"},
		{kubeVersion: "1.16", succeed: true, exp: "cis-1.4"},
		{kubeVersion: "unknown", succeed: false, exp: ""},
	}
	for _, c := range cases {
		rv := mapToCISVersion(kubeToCISMap, c.kubeVersion)
		if c.exp != rv {
			t.Errorf("mapToCISVersion kubeversion: %q Got %q expected %s", c.kubeVersion, rv, c.exp)
		}
	}
}

func TestLoadVersionMapping(t *testing.T) {
	setDefault := func(v *viper.Viper, key string, value interface{}) *viper.Viper {
		v.SetDefault(key, value)
		return v
	}

	viperWithData, err := loadConfigForTest()
	if err != nil {
		t.Fatalf("Unable to load config file %v", err)
	}

	cases := []struct {
		n       string
		v       *viper.Viper
		succeed bool
	}{
		{n: "empty", v: viper.New(), succeed: false},
		{
			n:       "novals",
			v:       setDefault(viper.New(), versionMapping, "novals"),
			succeed: false,
		},
		{
			n:       "good",
			v:       viperWithData,
			succeed: true,
		},
	}
	for _, c := range cases {
		rv, err := loadVersionMapping(c.v)
		if c.succeed {
			if err != nil {
				t.Errorf("[%q]-Unexpected error: %v", c.n, err)
			}

			if len(rv) == 0 {
				t.Errorf("[%q]-missing mapping value", c.n)
			}
		} else {
			if err == nil {
				t.Errorf("[%q]-Expected error but got none", c.n)
			}
		}
	}
}

func loadConfigForTest() (*viper.Viper, error) {
	viperWithData := viper.New()
	viperWithData.SetConfigFile(filepath.Join("..", cfgDir, "config.yaml"))
	if err := viperWithData.ReadInConfig(); err != nil {
		return nil, err
	}

	return viperWithData, nil
}
