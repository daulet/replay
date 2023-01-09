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
