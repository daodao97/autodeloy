package main

import (
	"fmt"
	"io"
	"os"
)

// CheckFileExists 检查指定路径的文件是否存在
// 返回 true 如果文件存在，false 如果文件不存在或者发生错误
func CheckFileExists(filePath string) bool {
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			// 文件不存在
			return false
		}
		// 发生其他错误
		return false
	}
	// 文件存在
	return true
}

func GetFileContent(filePath string) string {
	// 使用os.Open打开文件
	file, err := os.Open(filePath)
	if err != nil {
		// 错误处理
		fmt.Println("Error opening file:", err)
		return ""
	}
	defer file.Close() // 确保在函数返回前关闭文件

	// 使用io.ReadAll读取文件内容到内存
	content, err := io.ReadAll(file)
	if err != nil {
		// 错误处理
		fmt.Println("Error reading file:", err)
		return ""
	}

	// 将内容转换为字符串并打印
	return string(content)
}
