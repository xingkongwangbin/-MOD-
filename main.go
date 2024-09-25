package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed Dyson_Sphere_Program.zip
var embeddedZip embed.FS

func main() {
	fmt.Println("请加入QQ群:387590770 获取密码")

	for {
		var password string
		fmt.Print("请输入密码:")
		fmt.Scanln(&password)
		if password == "387590770" {
			fmt.Println("密码正确")
			break
		} else {
			fmt.Println("密码错误")
		}
	}

	var games_src string

	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 1366540`, registry.QUERY_VALUE)
	if err == nil {
		installLocation, _, err := key.GetStringValue("InstallLocation")
		if err == nil {
			games_src = installLocation
		}
	}
	defer key.Close()

	if games_src == "" {
		snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
		if err == nil {
			var entry windows.ProcessEntry32
			entry.Size = uint32(unsafe.Sizeof(entry))
			if err = windows.Process32First(snapshot, &entry); err == nil {
				for {
					processName := syscall.UTF16ToString(entry.ExeFile[:])
					if processName == "DSPGAME.exe" {
						fmt.Printf("Found DSPGAME.exe with PID: %d\n", entry.ProcessID)
						pid_path, err := getProcessPath(entry.ProcessID)
						if err == nil {
							games_src = strings.TrimSuffix(pid_path, `DSPGAME.exe`)

							// 获取进程句柄
							handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, entry.ProcessID)
							if err != nil {
								fmt.Println("无法打开进程句柄:", err)
								continue
							}
							// 尝试终止进程
							err = windows.TerminateProcess(handle, 1)
							if err != nil {
								fmt.Println("无法终止进程:", err)
							} else {
								fmt.Println("进程已终止")
							}
							windows.CloseHandle(handle)

							break
						}
					}

					if err = windows.Process32Next(snapshot, &entry); err != nil {
						if err == syscall.ERROR_NO_MORE_FILES {
							break
						}
						fmt.Println("Error getting next process:", err)
						return
					}

				}
			}

		}
		defer windows.CloseHandle(snapshot)

	}

	if games_src == "" {
		fmt.Print("无法获取游戏路径请关闭此软件并且打开游戏在打开软件会自动安装或者手动输入:")
		fmt.Scanln(&games_src)
	}

	for {
		_, err := os.Stat(games_src)
		if os.IsNotExist(err) {
			fmt.Println("游戏目录不存在")
			fmt.Print("请重新输入游戏路径:")
			fmt.Scanln(&games_src)
		} else {
			fmt.Print("\033[2J") // 清空整个屏幕
			fmt.Print("\033[H")  // 将光标移动到左上角
			fmt.Println("游戏目录:", games_src)
			break
		}
	}
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://dow.feishunet.com/Addons/Dyson_Sphere_Program/MOD/Version.txt", nil)
	if err != nil {
		fmt.Println("创建请求失败:", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("发送请求失败:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("读取响应失败:", err)
		return
	}

	latest_version := strings.TrimSpace(string(body))
	fmt.Println("最新的mod版本号:", latest_version)

	var mod_version string
	pluginsPath := filepath.Join(games_src, "BepInEx", "plugins")

	// 枚举 pluginsPath 下的所有子目录
	dirs, err := os.ReadDir(pluginsPath)
	if err == nil {
		for _, dir := range dirs {
			if dir.IsDir() {
				manifestPath := filepath.Join(pluginsPath, dir.Name(), "manifest.json")
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					continue
				}

				var manifest struct {
					Name          string `json:"name"`
					VersionNumber string `json:"version_number"`
				}

				err = json.Unmarshal(data, &manifest)
				if err != nil {
					continue
				}

				if manifest.Name == "NebulaMultiplayerMod" {
					mod_version = manifest.VersionNumber
					dirPath := filepath.Join(pluginsPath, dir.Name())
					os.RemoveAll(dirPath)
				}
			}
		}
	}
	if mod_version == "" {
		fmt.Println("未安装mod 直接安装最新的mod")
	} else {
		fmt.Println("当前mod版本号:", mod_version)
		if mod_version == latest_version {
			fmt.Println("mod已经是最新版本")
			fmt.Println("请按任意键关闭...")
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			return
		} else {
			fmt.Println("mod不是最新版本,开始安装mod")
		}
	}
	// 从 embed.FS 读取 ZIP 文件
	zipData, err := embeddedZip.ReadFile("Dyson_Sphere_Program.zip")
	if err != nil {
		fmt.Println("读取嵌入的 ZIP 文件失败:", err)
		return
	}

	// 创建 bytes.Reader
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		fmt.Println("创建 zip.Reader 失败:", err)
		return
	}

	for _, file := range zipReader.File {
		// 创建目标文件路径
		path := filepath.Join(games_src, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}

		// 创建文件的目录
		if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
			fmt.Println("创建目录失败:", err)
			continue
		}

		// 打开 zip 中的文件
		zippedFile, err := file.Open()
		if err != nil {
			fmt.Println("打开压缩文件失败:", err)
			continue
		}
		defer zippedFile.Close()

		// 创建目标文件
		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			fmt.Println("创建目标文件失败:", err)
			continue
		}
		defer targetFile.Close()

		// 复制文件内容
		if _, err := io.Copy(targetFile, zippedFile); err != nil {
			fmt.Println("写入文件失败:", err)
			continue
		}
	}

	fmt.Println("mod安装完成")
	fmt.Println("请按任意键关闭...")
	bufio.NewReader(os.Stdin).ReadBytes('\n')

}
func getProcessPath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	var n uint32 = 1024
	for {
		buf := make([]uint16, n)
		err = windows.QueryFullProcessImageName(handle, 0, &buf[0], &n)
		if err == nil {
			return syscall.UTF16ToString(buf[:n]), nil
		}
		if err.(syscall.Errno) != syscall.ERROR_INSUFFICIENT_BUFFER {
			return "", err
		}
		n *= 2
	}
}
