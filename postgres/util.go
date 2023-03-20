package postgres

func readN(ch <-chan byte, n int) []byte {
	bs := make([]byte, n)
	for i := 0; i < n; i++ {
		bs[i] = <-ch
	}
	return bs
}

func writeN(ch chan<- byte, bs []byte) {
	for _, b := range bs {
		ch <- b
	}
}

// read null delimited strings
func readStrings(buf []byte) []string {
	var res []string
	for i := 0; i < len(buf); {
		j := i
		for ; buf[j] != 0; j++ {
		}
		res = append(res, string(buf[i:j]))
		i += j + 1
	}
	return res
}

// write null delimited strings
func writeStrings(strs []string) []byte {
	var buf []byte
	for _, str := range strs {
		buf = append(buf, []byte(str)...)
		buf = append(buf, 0)
	}
	return buf
}
