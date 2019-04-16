

package filter

// Generic （通过） 结构体
type Generic struct {
	Str1, Str2, Str3 string
	Data             map[string]struct{}

	Fn func(data interface{})
}

// self =已注册，f =传入
// self = registered, f = incoming
func (self Generic) Compare(f Filter) bool {
	var strMatch, dataMatch = true, true

	filter := f.(Generic)
	if (len(self.Str1) > 0 && filter.Str1 != self.Str1) ||
		(len(self.Str2) > 0 && filter.Str2 != self.Str2) ||
		(len(self.Str3) > 0 && filter.Str3 != self.Str3) {
		strMatch = false
	}

	for k := range self.Data {
		if _, ok := filter.Data[k]; !ok {
			return false
		}
	}

	return strMatch && dataMatch
}

// 触发器
func (self Generic) Trigger(data interface{}) {
	self.Fn(data)
}
