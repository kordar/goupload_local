package goupload_local

import (
	"bufio"
	"fmt"
	"github.com/kordar/goupload"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

// FileExists 判断文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

// IsDirectory 判断给定路径是否是目录
func IsDirectory(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err // 返回错误，例如路径不存在等
	}
	return info.IsDir(), nil
}

// WalkDirWithPagination 遍历目录，分页获取文件路径
func WalkDirWithPagination(root string, abroot string, page int, pageSize int, subCount bool, f FilterDirItem) ([]goupload.BucketObject, int, error) {
	data := make([]goupload.BucketObject, 0, pageSize)
	var index, total = 0, 0
	offset := page - 1
	if offset < 0 {
		offset = 0
	}

	err := filepath.WalkDir(root, func(pathname string, d fs.DirEntry, err error) error {

		if index >= pageSize {
			return filepath.SkipDir
		}

		if err != nil {
			return err
		}

		// 排除当前目录
		if root == pathname {
			return nil
		}

		if f != nil && f(pathname, d) {
			return nil
		}

		info, err2 := d.Info()
		if err2 != nil {
			return nil
		}
		params := map[string]interface{}{
			"full_path": path.Join(root, d.Name()),
			"filename":  d.Name(),
		}
		if total >= offset*pageSize && index < pageSize {
			if d.IsDir() {
				if subCount {
					params["count"] = WalkDirCount(path.Join(root, d.Name()), f)
				}
				object := goupload.BucketObject{
					Path:         path.Join(abroot, d.Name()),
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					FileType:     "dir",
					Size:         info.Size(),
					Params:       params,
				}
				data = append(data, object)
			} else {
				data = append(data, goupload.BucketObject{
					Path:         path.Join(abroot, d.Name()),
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					Size:         info.Size(),
					FileType:     "file",
					FileExt:      path.Ext(pathname),
					Params:       params,
				})
			}
			index++
		}

		total++

		// 防止遍历子目录
		if d.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	return data, total, nil
}

// WalkDirCount 遍历目录，分页获取文件路径
func WalkDirCount(root string, f FilterDirItem) int {
	var total = 0
	_ = filepath.WalkDir(root, func(pathname string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		// 排除当前目录
		if root == pathname {
			return nil
		}

		if f != nil && f(pathname, d) {
			return nil
		}

		total++

		// 防止遍历子目录
		if d.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})

	return total
}

func TreeDir(root string, abroot string, next int, limit int, dep int, maxDep int, noleaf bool, subCount bool, f FilterDirItem) []goupload.BucketTreeObject {
	treeData := make([]goupload.BucketTreeObject, 0)
	_ = filepath.WalkDir(root, func(pathname string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		if root == pathname {
			return nil
		}

		if f != nil && f(pathname, d) {
			return nil
		}

		info, err2 := d.Info()
		if err2 != nil {
			return nil
		}

		subRoot := path.Join(root, d.Name())
		subAbRoot := path.Join(abroot, d.Name())
		params := map[string]interface{}{
			"full_path": subRoot,
			"filename":  d.Name(),
		}
		if d.IsDir() {
			object := goupload.BucketTreeObject{
				Path:         subAbRoot,
				LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
				FileType:     "dir",
				Size:         info.Size(),
				Params:       params,
				Children:     make([]goupload.BucketTreeObject, 0),
			}
			if maxDep <= 0 || dep < maxDep {
				object.Children = TreeDir(subRoot, subAbRoot, next, limit, dep+1, maxDep, noleaf, subCount, f)
			}
			if subCount {
				params["count"] = WalkDirCount(subRoot, f)
			}
			treeData = append(treeData, object)
			return filepath.SkipDir
		} else {
			if !noleaf {
				object := goupload.BucketTreeObject{
					Path:         subAbRoot,
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					Size:         info.Size(),
					FileType:     "file",
					FileExt:      path.Ext(pathname),
					Params:       params,
					Children:     make([]goupload.BucketTreeObject, 0),
				}
				treeData = append(treeData, object)
			}
		}
		return nil
	})
	return treeData
}

func AppendData(root string, fd io.Reader) (int, error) {
	file, err := os.OpenFile(root, os.O_WRONLY|os.O_APPEND, fs.ModePerm)
	if err != nil {
		return -1, fmt.Errorf("failed to open file")
	}
	defer file.Close()
	write := bufio.NewWriter(file)

	all, err2 := ioutil.ReadAll(fd)
	if err2 != nil {
		return -1, err2
	}

	nn, err3 := write.Write(all)
	if err3 != nil {
		return -1, err3
	}

	write.Flush()
	return nn, nil
}
