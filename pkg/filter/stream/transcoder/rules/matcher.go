/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rules

import (
	"context"
	"mosn.io/mosn/pkg/log"
	"mosn.io/mosn/pkg/types"
	"mosn.io/mosn/pkg/variable"
	"regexp"
)

type MatcherConfig struct {
	Headers   []Header   `json:"headers,omitempty"`
	Variables []Variable `json:"variables,omitempty"`
}

// Header specifies a set of headers that the rule should match on.
type Header struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
	Regex bool   `json:"regex,omitempty"`
}

// Variable specifies a set of variables that the rule should match on.
type Variable struct {
	Name     string `json:"name,omitempty"`
	Value    string `json:"value,omitempty"`
	Regex    bool   `json:"regex,omitempty"`
	Operator string `json:"operator,omitempty"` // support && and || operator
}

type RuleInfo struct {
	UpstreamProtocol    string                 `json:"upstream_protocol"`
	UpstreamSubProtocol string                 `json:"upstream_sub_protocol"`
	Description         string                 `json:"description"`
	Config              map[string]interface{} `json:"config"`
}

type VariableMatcher struct {
	Variables []Variable
}

const (
	AND string = "and"
	OR  string = "or"
)

func (vm VariableMatcher) matches(ctx context.Context, headers types.HeaderMap) bool {
	result := true
	walkVarName := ""
	lastMode := AND
	for _, v := range vm.Variables {
		walkVarName = v.Name
		curStepRes := false
		actual, _ := variable.GetVariableValue(ctx, v.Name)
		if v.Value != "" {
			curStepRes = v.Value == actual
		}

		if v.Regex {
			curStepRes, _ = regexp.MatchString(v.Operator, actual)
		}
		if lastMode == AND {
			result = result && curStepRes
		} else {
			result = curStepRes
		}
		if result {
			if v.Operator == OR {
				break
			}
		}
		lastMode = v.Operator
	}
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("variable transfer rule", "match success", walkVarName)
	}
	return result
}

type HeaderMatcher struct {
	Headers []Header
}

func (hm HeaderMatcher) matches(ctx context.Context, headers types.HeaderMap) bool {
	result := true
	walkVarName := ""
	for _, h := range hm.Headers {
		walkVarName = h.Name
		curStepRes := false
		value, _ := headers.Get(h.Name)
		curStepRes = h.Value == value
		if h.Regex {
			curStepRes, _ = regexp.MatchString(h.Value, value)
		}
		if !result {
			return result
		}
		result = curStepRes
	}
	if log.DefaultLogger.GetLogLevel() >= log.DEBUG {
		log.DefaultLogger.Debugf("headers transfer rule", "match success", walkVarName)
	}
	return result
}

type TransferRuleConfig struct {
	MatcherConfig *MatcherConfig `json:"macther_config"`
	RuleInfo      *RuleInfo      `json:"rule_info"`
}

func (tf *TransferRuleConfig) Matches(ctx context.Context, headers types.HeaderMap) (*RuleInfo, bool) {

	if tf.MatcherConfig == nil {
		log.DefaultLogger.Infof("[stream filter][transcoder][rules]matcher config is empty")
		return nil, false
	}

	result := false
	if len(tf.MatcherConfig.Variables) != 0 {
		result = VariableMatcher{tf.MatcherConfig.Variables}.matches(ctx, headers)

	} else if len(tf.MatcherConfig.Headers) != 0 {
		result = HeaderMatcher{tf.MatcherConfig.Headers}.matches(ctx, headers)
	}

	if result {
		return tf.RuleInfo, result
	}
	return nil, result
}
