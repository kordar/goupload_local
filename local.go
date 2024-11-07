package goupload_local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	logger "github.com/kordar/gologger"
	"github.com/kordar/goupload"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type LocalUploader struct {
	root       string
	bucketName string
	filter     FilterDirItem
}

func NewLocalUploader(root string, bucketName string, filter FilterDirItem) *LocalUploader {
	err := os.MkdirAll(path.Join(root, bucketName), fs.ModePerm)
	if err != nil {
		logger.Fatalf("[uploader local] failed to init root")
	}
	return &LocalUploader{root: root, bucketName: bucketName, filter: filter}
}

func (l *LocalUploader) realpath(p string, create bool) string {
	if create {
		dir := path.Join(l.root, l.bucketName, path.Dir(p))
		if !FileExists(dir) {
			_ = os.MkdirAll(dir, fs.ModePerm)
		}
	}
	return path.Join(l.root, l.bucketName, p)
}

func (l *LocalUploader) Name() string {
	return l.bucketName
}

func (l *LocalUploader) Driver() string {
	return "local"
}

func (l *LocalUploader) RemoteBuckets(ctx context.Context, args ...interface{}) []goupload.Bucket {
	// TODO 本地存储无远端bucket列表
	return []goupload.Bucket{}
}

func (l *LocalUploader) Get(ctx context.Context, name string, args ...interface{}) ([]byte, error) {
	realpath := l.realpath(name, false)
	return ioutil.ReadFile(realpath)
}

func (l *LocalUploader) GetToFile(ctx context.Context, name string, localPath string, args ...interface{}) error {
	if len(args) == 0 {
		return nil
	}

	switch v := args[0].(type) {
	case GetRemoteFd:
		if b, err := v(localPath); err == nil {
			reader := bytes.NewReader(b)
			return l.Put(ctx, name, reader, args...)
		}
	}
	return nil
}

func (l *LocalUploader) Put(ctx context.Context, name string, fd io.Reader, args ...interface{}) error {
	if fd == nil {
		return nil
	}

	realpath := l.realpath(name, true)
	out, err := os.Create(realpath)
	if err != nil {
		logger.Warnf("[%s,%s] put: failed to create file, %v", l.Driver(), l.Name(), err)
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer out.Close()

	// 将响应体的数据写入本地文件
	_, err = io.Copy(out, fd)
	if err != nil {
		logger.Warnf("[%s,%s] put: failed to save file, %v", l.Driver(), l.Name(), err)
		return fmt.Errorf("failed to save file: %v", err)
	}
	return nil
}

func (l *LocalUploader) PutString(ctx context.Context, name string, content string, args ...interface{}) error {
	reader := strings.NewReader(content)
	return l.Put(ctx, name, reader, args...)
}

func (l *LocalUploader) PutFromFile(ctx context.Context, name string, filePath string, args ...interface{}) error {
	realpath := l.realpath(filePath, true)
	fd, err := os.Open(realpath)
	if err != nil {
		logger.Warnf("[%s,%s] put: failed to open file, %v", l.Driver(), l.Name(), err)
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer fd.Close()
	return l.Put(ctx, name, fd, args...)
}

func (l *LocalUploader) List(ctx context.Context, dir string, next interface{}, limit int, args ...interface{}) ([]goupload.BucketObject, interface{}) {
	realpath := l.realpath(dir, false)
	if ok, _ := IsDirectory(realpath); ok {
		list, count, _ := WalkDirWithPagination(realpath, next.(int), limit, l.filter)
		return list, count
	}
	return []goupload.BucketObject{}, 0
}

func (l *LocalUploader) Tree(ctx context.Context, dir string, next interface{}, limit int, dep int, maxDep int, noleaf bool, args ...interface{}) []goupload.BucketTreeObject {
	realpath := l.realpath(dir, false)
	if ok, _ := IsDirectory(realpath); ok {
		return TreeDir(realpath, next.(int), limit, dep, maxDep, noleaf, l.filter)
	}
	return []goupload.BucketTreeObject{}
}

func (l *LocalUploader) Del(ctx context.Context, name string, args ...interface{}) error {
	if name == "" {
		return errors.New("please set the correct path name")
	}
	realpath := l.realpath(name, false)
	return os.Remove(realpath)
}

func (l *LocalUploader) DelAll(ctx context.Context, dir string, args ...interface{}) {
	if dir == "" {
		return
	}

	realpath := l.realpath(dir, false)
	files, err := ioutil.ReadDir(realpath)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			l.DelAll(ctx, path.Join(dir, file.Name()))
		}
		_ = l.Del(ctx, path.Join(dir, file.Name()), args...)
	}

	_ = l.Del(ctx, dir, args...)
}

func (l *LocalUploader) DelMulti(ctx context.Context, objects []goupload.BucketObject, args ...interface{}) error {
	for _, object := range objects {
		if object.FileType == "file" {
			_ = l.Del(ctx, object.Path, args...)
		}
		if object.FileType == "dir" {
			l.DelAll(ctx, object.Path)
		}
	}
	return nil
}

func (l *LocalUploader) IsExist(ctx context.Context, name string, args ...interface{}) (bool, error) {
	realpath := l.realpath(name, false)
	_, err := os.Stat(realpath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *LocalUploader) Copy(ctx context.Context, dest string, source string, args ...interface{}) error {
	return l.PutFromFile(ctx, dest, source, args...)
}

func (l *LocalUploader) Move(ctx context.Context, dest string, source string, args ...interface{}) error {
	if err := l.PutFromFile(ctx, dest, source, args...); err == nil {
		return l.Del(ctx, source, args...)
	} else {
		return err
	}
}

func (l *LocalUploader) Rename(ctx context.Context, dest string, source string, args ...interface{}) error {
	return l.Move(ctx, dest, source, args...)
}

func (l *LocalUploader) Append(ctx context.Context, name string, position int, fd io.Reader, args ...interface{}) (int, error) {
	realpath := l.realpath(name, false)
	return AppendData(realpath, fd)
}

func (l *LocalUploader) AppendString(ctx context.Context, name string, position int, content string, args ...interface{}) (int, error) {
	r := strings.NewReader(content)
	return l.Append(ctx, name, position, r, args...)
}
