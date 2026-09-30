package authz

import "strings"

func RuleKey(scope, action string) string {
	return strings.ToLower(strings.Join([]string{scope, action, ruleKeySuffix}, ruleKeySeparator))
}
