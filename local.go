package goupload_local

import (
	"bufio"
	"context"
	"fmt"
	logger "github.com/kordar/gologger"
	"github.com/kordar/goupload"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type GetRemoteFd = func(url string) (io.Reader, error)

type LocalUploader struct {
	root       string
	bucketName string
	filter     func(string, fs.DirEntry) bool
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func NewLocalUploader(root string, bucketName string, filter func(string, fs.DirEntry) bool) *LocalUploader {
	err := os.MkdirAll(path.Join(root, bucketName), fs.ModePerm)
	if err != nil {
		logger.Fatalf("[uploader local] failed to init root")
	}
	return &LocalUploader{root: root, bucketName: bucketName, filter: filter}
}

func (l *LocalUploader) getPath(p string, create bool) string {
	if create {
		dir := path.Join(l.root, l.bucketName, path.Dir(p))
		if !PathExists(dir) {
			_ = os.MkdirAll(dir, fs.ModePerm)
		}
	}
	return path.Join(l.root, l.bucketName, p)
}

func (l *LocalUploader) BucketName() string {
	return l.bucketName
}

func (l *LocalUploader) DriverName() string {
	return "local"
}

func (l *LocalUploader) RemoteBuckets(ctx context.Context, opt interface{}) []goupload.Bucket {
	return []goupload.Bucket{}
}

func (l *LocalUploader) Get(ctx context.Context, name string, opt interface{}, id ...string) ([]byte, error) {
	return ioutil.ReadFile(l.getPath(name, false))
}

func (l *LocalUploader) GetToFile(ctx context.Context, name string, localPath string, opt interface{}, id ...string) error {
	switch v := opt.(type) {
	case GetRemoteFd:
		if reader, err := v(localPath); err == nil {
			return l.Put(ctx, name, reader, opt)
		}
	}
	return nil
}

func (l *LocalUploader) PutFromFile(ctx context.Context, name string, filePath string, opt interface{}) error {
	open, err := os.Open(l.getPath(filePath, true))
	if err != nil {
		logger.Warnf("[%s, %s] put: failed to open file, %v", l.DriverName(), l.BucketName(), err)
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer open.Close()
	return l.Put(ctx, name, open, opt)
}

func (l *LocalUploader) Put(ctx context.Context, name string, fd io.Reader, opt interface{}) error {
	if fd == nil {
		return nil
	}
	// 创建本地文件
	out, err := os.Create(l.getPath(name, true))
	if err != nil {
		logger.Warnf("[%s, %s] put: failed to create file, %v", l.DriverName(), l.BucketName(), err)
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer out.Close()
	// 将响应体的数据写入本地文件
	_, err = io.Copy(out, fd)
	if err != nil {
		logger.Warnf("[%s, %s] put: failed to save file, %v", l.DriverName(), l.BucketName(), err)
		return fmt.Errorf("failed to save file: %v", err)
	}
	return nil
}

func (l *LocalUploader) PutString(ctx context.Context, name string, content string, opt interface{}) error {
	bytes := strings.NewReader(content)
	return l.Put(ctx, name, bytes, opt)
}

func (l *LocalUploader) List(ctx context.Context, dir string, next string, limit int, opt interface{}) ([]goupload.BucketObject, string) {
	realPath := l.getPath(dir, false)
	data := make([]goupload.BucketObject, 0, limit)
	var index, total = 0, 0
	offset, err := strconv.Atoi(next)
	if err != nil {
		return []goupload.BucketObject{}, ""
	}

	_ = filepath.WalkDir(realPath, func(pathname string, d fs.DirEntry, err error) error {
		if index >= limit {
			return fmt.Errorf("over scan")
		}

		if err != nil {
			logger.Warnf("[%s,%s] %v", l.DriverName(), l.BucketName(), err)
			return err
		}

		if strings.HasSuffix(pathname, dir) {
			return nil
		}

		if l.filter != nil && l.filter(pathname, d) {
			return nil
		}

		info, err2 := d.Info()
		if err2 != nil {
			return nil
		}

		if total >= offset && index < limit {
			if d.IsDir() {
				data = append(data, goupload.BucketObject{
					Path:         path.Join(dir, d.Name()),
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					FileType:     "dir",
					Size:         info.Size(),
				})
			} else {
				data = append(data, goupload.BucketObject{
					Path:         path.Join(dir, d.Name()),
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					Size:         info.Size(),
					FileType:     "file",
					FileExt:      path.Ext(pathname),
				})
			}
			index++
		}

		total++
		if d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})

	return data, ""
}

func (l *LocalUploader) Del(ctx context.Context, name string, opt interface{}) error {
	if name == "" {
		return nil
	}
	getPath := l.getPath(name, false)
	return os.Remove(getPath)
}

func (l *LocalUploader) DelAll(ctx context.Context, dir string) {
	if dir == "" {
		return
	}

	files, err := ioutil.ReadDir(l.getPath(dir, false))
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			l.DelAll(ctx, path.Join(dir, file.Name()))
		}
		_ = l.Del(ctx, path.Join(dir, file.Name()), nil)
	}

	_ = l.Del(ctx, dir, nil)
}

func (l *LocalUploader) DelMulti(ctx context.Context, objects []goupload.BucketObject) error {
	for _, object := range objects {
		if object.FileType == "file" {
			_ = l.Del(ctx, object.Path, nil)
		}
		if object.FileType == "dir" {
			l.DelAll(ctx, object.Path)
		}
	}
	return nil
}

func (l *LocalUploader) IsExist(ctx context.Context, name string, id ...string) (bool, error) {
	_, err := os.Stat(l.getPath(name, false))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *LocalUploader) Copy(ctx context.Context, dest string, source string, opt interface{}) error {
	return l.PutFromFile(ctx, dest, source, opt)
}

func (l *LocalUploader) Move(ctx context.Context, dest string, source string, opt interface{}) error {
	if err := l.PutFromFile(ctx, dest, source, opt); err == nil {
		return l.Del(ctx, source, opt)
	} else {
		return err
	}
}

func (l *LocalUploader) Rename(ctx context.Context, dest string, source string, opt interface{}) error {
	return l.Move(ctx, dest, source, opt)
}

func (l *LocalUploader) Tree(ctx context.Context, dir string, next string, limit int, dep int, maxDep int, noleaf bool) []goupload.BucketTreeObject {
	treeData := make([]goupload.BucketTreeObject, 0)
	realPath := l.getPath(dir, false)
	_ = filepath.WalkDir(realPath, func(pathname string, d fs.DirEntry, err error) error {

		if err != nil {
			logger.Warnf("[%s,%s] %v", l.DriverName(), l.BucketName(), err)
			return err
		}

		if strings.HasSuffix(pathname, dir) {
			return nil
		}

		if l.filter != nil && l.filter(pathname, d) {
			return nil
		}

		info, err2 := d.Info()
		if err2 != nil {
			return nil
		}

		sub := path.Join(dir, d.Name())
		if d.IsDir() {
			object := goupload.BucketTreeObject{
				Item: goupload.BucketObject{
					Path:         sub,
					LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
					FileType:     "dir",
					Size:         info.Size(),
				},
				Children: make([]goupload.BucketTreeObject, 0),
			}
			if maxDep <= 0 || dep < maxDep {
				object.Children = l.Tree(ctx, sub, next, limit, dep+1, maxDep, noleaf)
			}
			treeData = append(treeData, object)
			return filepath.SkipDir
		} else {
			if !noleaf {
				object := goupload.BucketTreeObject{
					Item: goupload.BucketObject{
						Path:         sub,
						LastModified: info.ModTime().Format("2006-01-02 15:04:05"),
						Size:         info.Size(),
						FileType:     "file",
						FileExt:      path.Ext(pathname),
					},
					Children: make([]goupload.BucketTreeObject, 0),
				}
				treeData = append(treeData, object)
			}
		}
		return nil
	})
	return treeData
}

func (l *LocalUploader) Append(ctx context.Context, name string, position int, r io.Reader, opt interface{}) (int, error) {
	file, err := os.OpenFile(l.getPath(name, false), os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		logger.Warnf("[%s, %s] append: failed to open file", l.DriverName(), l.BucketName())
		return -1, fmt.Errorf("failed to open file")
	}
	defer file.Close()
	write := bufio.NewWriter(file)

	all, err2 := ioutil.ReadAll(r)
	if err2 != nil {
		logger.Warnf("[%s, %s] append: io.Reader error, %v", l.DriverName(), l.BucketName(), err2)
		return -1, err2
	}

	nn, err3 := write.Write(all)
	if err3 != nil {
		logger.Warnf("[%s, %s] append: failed to write file, %v", l.DriverName(), l.BucketName(), err3)
		return -1, err3
	}

	write.Flush()
	return nn, nil
}

func (l *LocalUploader) AppendString(ctx context.Context, name string, position int, content string, opt interface{}) (int, error) {
	r := strings.NewReader(content)
	return l.Append(ctx, name, position, r, opt)
}
