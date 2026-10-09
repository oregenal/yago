// Yandex API https://yandex.ru/dev/disk-api/doc/ru/concepts/quickstart
// https://yandex.ru/dev/disk/rest/

// TODO Usage message
// TODO Error handling
// TODO Response error handling
// TODO Prettier sizes output
// TODO Prettier error messages
// TODO handle multiple files deletion

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

func (d *Disk) Usage(appName string) {
	fmt.Printf("Yandex Disk CLI utility\n")
	fmt.Printf("Usage:\n")
	fmt.Printf("    %s help                            - show this help message\n", appName)
	fmt.Printf("    %s                                 - show disk space information\n", appName)
	fmt.Printf("    %s ls [dir]                        - list Yandex Disk directories\n", appName)
	fmt.Printf("    %s down <path/file>                - download file\n", appName)
	fmt.Printf("    %s up <path/file> <disk_path/file> - upload file\n", appName)
	fmt.Printf("    %s mkdir <path/dir>                - create directory\n", appName)
	fmt.Printf("    %s rm <path/dir|file>              - remove directory or file\n", appName)
	fmt.Printf("    %s token <OAuth_token>             - set Yandex Disk OAuth token\n", appName)
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
	path = d.normalizePath(path)
	path, err := url.JoinPath("disk:/", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

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

	dirName = d.normalizePath(dirName)

	query := url.Values{}
	query.Add("path", dirName)
	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(http.MethodPut, urlRequest.String())
	defer resp.Body.Close()

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusCreated {
		msg := result["message"]
		fmt.Fprintf(os.Stderr, "%v %v\n", resp.StatusCode, msg)
		os.Exit(1)
	}
}

func (d *Disk) Remove(path string) {
	urlRequest, err := url.Parse(baseUrl)
	urlRequest = urlRequest.JoinPath("resources")

	path = d.normalizePath(path)
	path, err = url.JoinPath("disk:/", path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	query := url.Values{}
	query.Add("path", path)
	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(http.MethodDelete, urlRequest.String())
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		var result map[string]any
		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
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
	fileName = d.normalizePath(fileName)
	fileName, err := url.JoinPath("disk:/", fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	values := url.Values{}
	values.Add("path", fileName)
	href := d.Request(http.MethodGet, "/resources/download", values)

	fout, err := os.Create(filepath.Base(fileName))
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer fout.Close()

	resp := d.doRequest(href["method"].(string), href["href"].(string))
	defer resp.Body.Close()

	_, err = io.Copy(fout, resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func (d *Disk) Upload(fileName string, path string) {
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	fStat, err := file.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if fStat.IsDir() {
		fmt.Fprintf(
			os.Stderr,
			"only file can be uploaded, but %v is directory\n",
			fileName)
		os.Exit(1)
	}

	path = d.normalizePath(path)
	path, err = url.JoinPath("disk:/", path, fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	values := url.Values{}
	values.Add("path", path)

	href := d.Request(http.MethodGet, "resources/upload", values)

	url := href["href"].(string)
	req, err := http.NewRequest(http.MethodPut, url, file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	req.ContentLength = fStat.Size()

	resp, err := d.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Fprintf(os.Stderr, "upload fail\n")
		os.Exit(1)
	}
}

func (d *Disk) Request(method string, APIpath string, query url.Values) Response {
	urlRequest, err := url.Parse(baseUrl)
	urlRequest = urlRequest.JoinPath(APIpath)

	urlRequest.RawQuery = query.Encode()

	resp := d.doRequest(method, urlRequest.String())
	defer resp.Body.Close()

	var result map[string]any
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "%v\n", result["message"])
		os.Exit(1)
	}

	return result
}

func (d *Disk) doRequest(method string, urlStr string) *http.Response {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", d.Token)

	resp, err := d.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return resp
}

func (d *Disk) normalizePath(path string) string {
	path = strings.TrimPrefix(path, "disk:")
	path = strings.TrimPrefix(path, "/")
	
	return path
}

func mustToken(appName string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
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
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	filePath := filepath.Join(homeDir, tokenFile)

	data := []byte(token)

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func usageError(args []string) {
	appName := filepath.Base(args[0])

	fmt.Fprintf(os.Stderr, "Unknown command: %s %s\n", appName, strings.Join(args[1:], " "))
	fmt.Fprintf(os.Stderr, "For more info use: %s help\n", appName)
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
		case "help":
			disk.Usage(filepath.Base(os.Args[0]))
		default:
			usageError(os.Args)
			os.Exit(1)
		}
	case 3:
		switch os.Args[1] {
		case "ls":
			disk.List(os.Args[2])
		case "down":
			disk.Download(os.Args[2])
		case "up":
			disk.Upload(os.Args[2], "") 
		case "mkdir":
			disk.MakeDir(os.Args[2])
		case "rm":
			disk.Remove(os.Args[2])
		case "token":
			newToken(os.Args[2])
		default:
			usageError(os.Args)
			os.Exit(1)
		}
	case 4:
		switch os.Args[1] {
		case "up":
			disk.Upload(os.Args[2], os.Args[3])
		default:
			usageError(os.Args)
			os.Exit(1)
		}
	default:
		usageError(os.Args)
		os.Exit(1)
	}
}
