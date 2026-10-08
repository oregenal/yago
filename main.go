// Yandex API https://yandex.ru/dev/disk-api/doc/ru/concepts/quickstart
// https://yandex.ru/dev/disk/rest/

// TODO Usage message
// TODO Error handling
// TODO Response error handling
// TODO Prettier sizes output
// TODO Prettier error messages

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	baseUrl   = "https://cloud-api.yandex.net/v1/disk"
	tokenFile = ".yago"
	listLimit = 50
)

type Disk struct {
	http.Client
	Token string
}

type Response map[string]any

func mustToken(appName string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "token fail: %v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(homeDir, tokenFile)

	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "first you must provide token\n")
		fmt.Fprintf(os.Stderr, "USAGE: %s token <token>\n", appName)
		os.Exit(1)
	}

	return strings.TrimSpace(string(data))
}

func newToken(token string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "new token fail: %v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(homeDir, tokenFile)

	data := []byte(token)

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new token fail: %v\n", err)
		os.Exit(1)
	}
}

func (d *Disk) Info() {
	disk_info := d.Request(http.MethodGet, "", nil)

	const mib = 1024 * 1024
	total_space := disk_info["total_space"].(float64) / mib
	used_space := disk_info["used_space"].(float64) / mib
	trash_size := disk_info["trash_size"].(float64) / mib
	free_space := total_space - used_space - trash_size

	fmt.Printf("total space: %.0f Mib\n", total_space)
	fmt.Printf("free space:  %.0f Mib\n", free_space)
	fmt.Printf("used space:  %.0f Mib\n", used_space)
	fmt.Printf("trash space: %.0f Mib\n", trash_size)
}

func (d *Disk) List(path string) {
	path = "disk:/" + path

	values := url.Values{}
	values.Add("limit", strconv.Itoa(listLimit))
	values.Add("path", path)

	folder := d.Request(http.MethodGet, "/resources", values)

	if folder["_embedded"] != nil {
		items := folder["_embedded"].(map[string]any)["items"]
		for _, v := range items.([]any) {
			fmt.Println(v.(map[string]any)["path"])
		}
	} else {
		fmt.Printf("%s is a file\n", path)
	}
}

func (d *Disk) MakeDir(dirName string) {
	urlRequest, err := url.Parse(baseUrl)
	urlRequest = urlRequest.JoinPath("resources")

	query := url.Values{}
	query.Add("path", dirName)
	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(http.MethodPut, urlRequest.String())
	defer resp.Body.Close()

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir fail: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusCreated {
		msg := result["message"]
		fmt.Fprintf(os.Stderr, "mkdir fail: %v %v\n", resp.StatusCode, msg)
		os.Exit(1)
	}
}

func (d *Disk) RemoveDir(dirName string) {
	urlRequest, err := url.Parse(baseUrl)
	urlRequest = urlRequest.JoinPath("resources")

	query := url.Values{}
	query.Add("path", dirName)
	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(http.MethodDelete, urlRequest.String())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "rm fail: %v\n", err)
			os.Exit(1)
		}

		if resp.StatusCode != http.StatusAccepted {
			msg := result["message"]
			fmt.Fprintf(os.Stderr, "rm fail: %v %v\n", resp.StatusCode, msg)
			os.Exit(1)
		}
	}
}

func (d *Disk) Download(fileName string) {
	path := "disk:/" + fileName

	values := url.Values{}
	values.Add("path", path)
	href := d.Request(http.MethodGet, "/resources/download", values)

	fout, err := os.Create(filepath.Base(fileName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "download fail %v\n", err)
		os.Exit(1)
	}
	defer fout.Close()

	resp := d.doRequest(href["method"].(string), href["href"].(string))
	defer resp.Body.Close()

	_, err = io.Copy(fout, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download fail %v\n", err)
		os.Exit(1)
	}
}

func (d *Disk) Upload(fileName string, path string) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload fail %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	path = "disk:/" + path

	values := url.Values{}
	values.Add("path", path)

	href := d.Request(http.MethodGet, "resources/upload", values)

	url := href["href"].(string)
	req, err := http.NewRequest(http.MethodPut, url, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload fail: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	fStat, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload fail: %v\n", err)
		os.Exit(1)
	}
	req.ContentLength = fStat.Size()

	resp, err := d.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "upload fail: %v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Fprintf(os.Stderr, "upload fail")
		os.Exit(1)
	}
}

func (d *Disk) Request(method string, path string, query url.Values) Response {
	urlRequest, err := url.Parse(baseUrl)
	urlRequest = urlRequest.JoinPath(path)

	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(method, urlRequest.String())
	defer resp.Body.Close()

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request fail: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Request fail: %v\n", result["message"])
		os.Exit(1)
	}

	return result
}

func (d *Disk) doRequest(method string, urlStr string) *http.Response {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doRequest fail %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", d.Token)

	resp, err := d.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "doRequest fail %v\n", err)
		os.Exit(1)
	}

	return resp
}

func main() {
	if len(os.Args) == 3 {
		if os.Args[1] == "token" {
			newToken(os.Args[2])
			fmt.Println("new token set")
			os.Exit(0)
		}
	}

	disk := &Disk{
		Token: "OAuth " + mustToken(os.Args[0]),
	}

	switch len(os.Args) {
	case 1:
		disk.Info()
	case 2:
		switch os.Args[1] {
		case "ls":
			disk.List("")
		default:
			fmt.Fprintf(os.Stderr, "Unknown command\n")
			os.Exit(1)
		}
	case 3:
		switch os.Args[1] {
		case "ls":
			disk.List(os.Args[2])
		case "down":
			disk.Download(os.Args[2])
		case "mkdir":
			disk.MakeDir(os.Args[2])
		case "rm":
			disk.RemoveDir(os.Args[2])
		case "token":
			newToken(os.Args[2])
		default:
			fmt.Fprintf(os.Stderr, "Unknown command\n")
			os.Exit(1)
		}
	case 4:
		switch os.Args[1] {
		case "up":
			disk.Upload(os.Args[2], os.Args[3])
		default:
			fmt.Fprintf(os.Stderr, "Unknown command\n")
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command\n")
		os.Exit(1)
	}
}
