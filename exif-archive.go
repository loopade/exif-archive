// 将照片按照拍摄日期归档到指定文件夹
package main

import (
	"flag"
	"fmt"

	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/djherbis/times"
	"github.com/dsoprea/go-exif/v3"
)

// 定义可用的文件格式
var EXTENSIONS map[string]bool = map[string]bool{
	".jpg":  true,
	".png":  true,
	".webp": true,
	".gif":  true,
	".mp4":  true,
	".mkv":  true,
	".jpeg": true,
	".heic": true,
	".mov":  true,
	".tiff": true,
}

// 定义文件时间结构体，包括创建时间、exif时间、文件名时间
type fileTimes struct {
	BirthTime    time.Time
	ExifTime     time.Time
	FileNameTime time.Time
}

// 按多个分隔符分割字符串
func SplitAny(s string, seps string) []string {

	splitter := func(r rune) bool {
		return strings.ContainsRune(seps, r)
	}
	return strings.FieldsFunc(s, splitter)
}

// 读取exif拍摄时间
func readExifTime(filePath string) (time.Time, error) {
	opt := exif.ScanOptions{}
	dt, err := exif.SearchFileAndExtractExif(filePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("读取exif失败%s", err)
	}
	ets, _, err := exif.GetFlatExifData(dt, &opt)
	if err != nil {
		return time.Time{}, fmt.Errorf("读取ets失败%s", err)
	}
	for _, et := range ets {
		if et.TagName == "DateTimeOriginal" {
			tm, err := time.Parse("2006:01:02 15:04:05", et.Value.(string))
			if err != nil {
				return time.Time{}, fmt.Errorf("exif中拍摄日期格式转换失败%s", err)
			}
			return tm, nil
		}
	}
	return time.Time{}, fmt.Errorf("exif中不包含拍摄日期")
}

// 读取文件名中的时间或时间戳
func readFileNameTime(fileName string) (time.Time, error) {
	fileNameBase := filepath.Base(fileName[:len(fileName)-len(filepath.Ext(fileName))])
	fileNameParts := SplitAny(fileNameBase, "_.- ")
	for i, part := range fileNameParts {
		if len(part) == 13 || (len(part) == 21 && part[:8] == "mmexport") {
			// wx_camera_1689324886317.jpg or mmexport1622020005757.jpg
			timestampStr := part
			if len(part) == 21 {
				timestampStr = part[8:]
			}
			timestampInt, err := strconv.ParseInt(timestampStr, 10, 64)
			if err == nil {
				return time.Unix(timestampInt/1000, 0), nil
			} else {
				log.Printf("【注意】%s文件名中的时间戳无效\n", fileName)
			}
		} else if len(part) == 17 || len(part) == 14 {
			// 20210526085304575.jpg
			datetimeStr := part
			tm, err := time.Parse("20060102150405", datetimeStr[:14])
			if err == nil {
				return tm, nil
			} else {
				log.Printf("【注意】%s文件名中的17位时间无效\n", fileName)
			}
		} else if len(part) == 8 && i < len(fileNameParts)-1 && len(fileNameParts[i+1]) == 6 {
			// Snapshot_20230626_113855_appname.mp4
			dateStr := part
			timeStr := fileNameParts[i+1]
			tm, err := time.Parse("20060102 150405", dateStr+" "+timeStr)
			if err == nil {
				return tm, nil
			} else {
				log.Printf("【注意】%s文件名中的8位日期无效\n", fileName)
			}
		} else if len(part) == 4 && i < len(fileNameParts)-2 && len(fileNameParts[i+1]) == 2 && len(fileNameParts[i+2]) == 2 {
			// 2021-06-26_11-38-55.mp4 or 2021-06-26_230139.jpg
			yearStr := part
			monthStr := fileNameParts[i+1]
			dayStr := fileNameParts[i+2]
			tm, err := time.Parse("2006-01-02_150405", yearStr+"-"+monthStr+"-"+dayStr+"_000000")
			if err == nil {
				return tm, nil
			} else {
				log.Printf("【注意】%s文件名中的日期无效\n", fileName)
			}
		}
	}
	return time.Time{}, fmt.Errorf("文件%s中不包含时间", fileName)
}

func main() {
	// 读取命令行参数origin和target
	originPath := flag.String("origin", "", "照片文件路径")
	targetPath := flag.String("target", "", "归档文件路径")
	flag.Parse()
	// 打印origin_path、target_path
	log.Println("照片文件路径:", *originPath)
	log.Println("归档文件路径:", *targetPath)
	// 判断是否有效路径
	if _, err := os.Stat(*originPath); err != nil {
		log.Fatalf("照片文件路径无效")
	}
	if _, err := os.Stat(*targetPath); err != nil {
		log.Fatalf("归档文件路径无效")
	}

	// 读取照片文件夹下所有文件
	files, err := os.ReadDir(*originPath)
	if err != nil {
		log.Fatal("读取照片文件夹失败")
	}
	for _, file := range files {
		fileName := file.Name()
		currentFile := filepath.Join(*originPath, fileName)
		if file.IsDir() {
			// 判断是否为文件夹
			continue
		} else if _, ok := EXTENSIONS[strings.ToLower(filepath.Ext(fileName))]; !ok {
			// 判断文件后缀是否在EXTENSIONS中
			log.Printf("【注意】%s 文件非可用格式\n", fileName)
			continue
		} else {
			fileTime := fileTimes{}

			// 读取文件创建日期
			t, err := times.Stat(currentFile)
			if err == nil && t.HasBirthTime() {
				fileTime.BirthTime = t.BirthTime()
			}
			// 读取exif拍摄日期
			exifTime, err := readExifTime(currentFile)
			if err == nil {
				fileTime.ExifTime = exifTime
			}
			// 读取文件名中的时间
			fileNameTime, err := readFileNameTime(fileName)
			if err == nil {
				fileTime.FileNameTime = fileNameTime
			}

			// 按照优先级选择时间：exif > 文件名时间 > 创建时间
			var finalTime time.Time
			if !fileTime.ExifTime.IsZero() {
				finalTime = fileTime.ExifTime
			} else if !fileTime.FileNameTime.IsZero() {
				finalTime = fileTime.FileNameTime
			} else if !fileTime.BirthTime.IsZero() {
				log.Printf("【注意】%s 文件只有创建时间 %s\n", fileName, fileTime.BirthTime)
				finalTime = fileTime.BirthTime
			} else {
				log.Fatalf("【错误】%s 文件时间信息异常\n", fileName)
			}

			// 创建目标文件夹
			targetDir := filepath.Join(*targetPath, finalTime.Format("2006-01"))
			if _, err := os.Stat(targetDir); os.IsNotExist(err) {
				log.Printf("创建目标文件夹 %s\n", targetDir)
				os.Mkdir(targetDir, os.ModePerm)
			}

			// 移动文件
			targetFile := filepath.Join(targetDir, fileName)
			if _, err := os.Stat(targetFile); err == nil {
				// 比较两个文件
				same, err := CompareFiles(currentFile, targetFile)
				if err != nil {
					log.Fatalf("【错误】比较文件失败 %s -> %s\n", currentFile, targetFile)
				}
				if same {
					log.Printf("【注意】%s 文件已存在且内容相同，删除文件\n", targetFile)
					err := os.Remove(currentFile)
					if err != nil {
						log.Fatalf("【错误】删除文件失败 %s\n", currentFile)
					}
				} else {
					log.Fatalf("【错误】%s 文件已存在且内容不同\n", targetFile)
				}
			} else {
				err := os.Rename(currentFile, targetFile)
				if err != nil {
					log.Fatalf("【错误】移动文件失败 %s -> %s\n", currentFile, targetFile)
				}
				log.Printf("移动文件 %s -> %s\n", currentFile, targetFile)
			}
		}
	}

	// 输入回车键退出
	fmt.Println("Press \"Enter\" to exit ...")
	fmt.Scanln()
}
