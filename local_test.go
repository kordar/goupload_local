package goupload_local_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	logger "github.com/kordar/gologger"
	"github.com/kordar/goupload"
	"github.com/kordar/goupload_local"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"testing"
)

var (
	uploader = goupload_local.NewLocalUploader("/Users/mac/Pictures/bucket", "test", func(s string, entry fs.DirEntry) bool {
		base := path.Base(s)
		return strings.HasPrefix(base, ".")
	})
	ctx = context.Background()
	mgr = goupload.NewManagerWithUploader(uploader)
)

func TestLocalUploader_GetToFile(t *testing.T) {
	// 发送 GET 请求获取文件内容
	_ = uploader.GetToFile(ctx, "baidu.html", "https://www.baidu.com", func(url string) (io.Reader, error) {

		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to request file: %v", err)
		}
		defer resp.Body.Close()

		// 检查是否请求成功
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download file: %s", resp.Status)
		}

		body, _ := ioutil.ReadAll(resp.Body)
		reader := bytes.NewReader(body)

		return reader, nil
	})
}

func TestLocalUploader_PutString(t *testing.T) {
	err := uploader.PutString(ctx, "/AAA/BB/CC/x4.txt", "AAA", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_Copy(t *testing.T) {
	err := uploader.Copy(ctx, "m.txt", "x.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_Move(t *testing.T) {
	err := uploader.Move(ctx, "m2.txt", "m.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_AppendString(t *testing.T) {
	pos, err := uploader.AppendString(ctx, "m3.txt", 0, "\nMMMM\n", nil)
	logger.Infof("=============%v, %v", err, pos)
}

func TestLocalUploader_Del(t *testing.T) {
	err := uploader.Del(ctx, "AA/BB/t.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_DelAll(t *testing.T) {
	uploader.DelAll(ctx, "")
}

func TestLocalUploader_List(t *testing.T) {
	list, next := uploader.List(ctx, "AAA/BB/CC", "0", 1000, nil)
	marshal, _ := json.Marshal(list)
	logger.Infof("-----------%v,----%v", string(marshal), next)
}

func TestLocalUploader_Tree(t *testing.T) {
	list := uploader.Tree(ctx, "AAA", "", 100, 0, 1, false)
	marshal, _ := json.Marshal(list)
	logger.Infof("-----------%v", string(marshal))
}

/**
 * **************************************************
 * * manager 测试
 * **************************************************
 */

func TestLocalUploader_GetToFile_Mgr(t *testing.T) {
	// 发送 GET 请求获取文件内容
	_ = mgr.GetToFile("test", "baidu.html", "https://www.baidu.com", func(url string) (io.Reader, error) {

		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to request file: %v", err)
		}
		defer resp.Body.Close()

		// 检查是否请求成功
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to download file: %s", resp.Status)
		}

		body, _ := ioutil.ReadAll(resp.Body)
		reader := bytes.NewReader(body)

		return reader, nil
	})
}

func TestLocalUploader_PutString_Mgr(t *testing.T) {
	err := mgr.PutString("test", "/AA/BB/CC/x4.txt", "AAA", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_Copy_Mgr(t *testing.T) {
	err := mgr.Copy("test", "m.txt", "/AA/BB/CC/x4.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_Move_Mgr(t *testing.T) {
	err := mgr.Move("test", "m2.txt", "m.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_AppendString_Mgr(t *testing.T) {
	pos, err := mgr.AppendString("test", "m2.txt", 20, "cccc\n", nil)
	logger.Infof("=============%v, %v", err, pos)
}

func TestLocalUploader_Del_Mgr(t *testing.T) {
	err := mgr.Del("test", "m2.txt", nil)
	logger.Infof("=============%v", err)
}

func TestLocalUploader_DelAll_Mgr(t *testing.T) {
	mgr.DelAll("test", "AA")
}

func TestLocalUploader_List_Mgr(t *testing.T) {
	list, next := mgr.List("test", "AA", "0", 1000, nil)
	marshal, _ := json.Marshal(list)
	logger.Infof("-----------%v,----%v", string(marshal), next)
}

func TestLocalUploader_Tree_Mgr(t *testing.T) {
	list := mgr.Tree("test", "AA", "", 100, 0, 0, false)
	marshal, _ := json.Marshal(list)
	logger.Infof("-----------%v", string(marshal))
}
