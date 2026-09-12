package app

import "os"

// DirExists 判断目录是否存在。
func DirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
