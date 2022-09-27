package elastic

import (
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var (
	serviceNameInvalidRegexp = regexp.MustCompile("[^a-zA-Z0-9 _-]")
	labelKeyReplacer         = strings.NewReplacer(`.`, `_`, `*`, `_`, `"`, `_`)
)

func ifaceAttributeValue(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeString:
		return truncate(v.StringVal())
	case pcommon.ValueTypeInt:
		return v.IntVal()
	case pcommon.ValueTypeDouble:
		return v.DoubleVal()
	case pcommon.ValueTypeBool:
		return v.BoolVal()
	}
	return nil
}

func cleanServiceName(name string) string {
	return serviceNameInvalidRegexp.ReplaceAllString(truncate(name), "_")
}

func cleanLabelKey(k string) string {
	return labelKeyReplacer.Replace(truncate(k))
}

func truncate(s string) string {
	const maxRunes = 1024
	var j int
	for i := range s {
		if j == maxRunes {
			return s[:i]
		}
		j++
	}
	return s
}
