package postgres

import (
	"sort"
)

func deterministicStartup(buf []byte) []byte {
	// key-value pairs
	m := make(map[string]string)
	for i := 8; buf[i] != 0; { // first 8 bytes are length and protocol version
		var ss []string
		for k := 0; k < 2; k++ {
			var str []byte
			for j := i; buf[j] != 0; j++ {
				str = append(str, buf[j])
			}
			ss = append(ss, string(str))
			i += len(str) + 1
		}
		m[ss[0]] = ss[1]
	}

	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var output []byte
	output = append(output, buf[:8]...) // length + protocol version
	for _, k := range keys {
		output = append(output, []byte(k)...)
		output = append(output, 0) // NUL
		output = append(output, []byte(m[k])...)
		output = append(output, 0) // NUL
	}
	output = append(output, 0) // NUL
	return output
}
