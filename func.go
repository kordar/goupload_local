package goupload_local

import "io/fs"

// GetRemoteFd 获取远端请求的字节数据
type GetRemoteFd = func(url string) ([]byte, error)

// FilterDirItem 过滤目录元素
type FilterDirItem = func(string, fs.DirEntry) bool
