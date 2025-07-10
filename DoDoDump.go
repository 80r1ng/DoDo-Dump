package main

import (
	"flag"
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/jarvanstack/mysqldump"
	"os"
	"time"
)

var (
	LOGO = `
    ____        ____              ____                      
   / __ \____  / __ \____        / __ \__  ______ ___  ____ 
  / / / / __ \/ / / / __ \______/ / / / / / / __  __ \/ __ \
 / /_/ / /_/ / /_/ / /_/ /_____/ /_/ / /_/ / / / / / / /_/ /
/_____/\____/_____/\____/     /_____/\__,_/_/ /_/ /_/ .___/ 
                                                   /_/
                                             ---数据库转储备份工具
                                             -h 查看详细使用方法
`
	DB_NAME         string
	USER_NAME       string
	USER_PASSWD     string
	ADDR            string
	PORT            string
	OUTPUT_FILE     string
	UPLOAD_OSS_FLAG bool

	// OSS 参数
	OSS_ENDPOINT   string
	OSS_AK         string
	OSS_SK         string
	OSS_BUCKETNAME string

	CONF = &Config{}
)

type Config struct {
	Endpoint   string
	AK         string
	SK         string
	BucketName string
}

func UploadFile(filename string) (downloadURL string, err error) {
	client, err := oss.New(CONF.Endpoint, CONF.AK, CONF.SK)
	if err != nil {
		return "", fmt.Errorf("new client error: %s", err)
	}

	bucket, err := client.Bucket(CONF.BucketName)
	if err != nil {
		return "", fmt.Errorf("get bucket %s error: %s", CONF.BucketName, err)
	}

	err = bucket.PutObjectFromFile(filename, filename)
	if err != nil {
		return "", fmt.Errorf("upload file %s error: %s", filename, err)
	}

	return bucket.SignURL(filename, oss.HTTPGet, 60*60*24*3)
}

func formatBytes(bytes int64) string {
	const (
		_  = iota
		KB = 1 << (10 * iota)
		MB
		GB
		TB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%d bytes", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	default:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	}
}

func init() {
	fmt.Println(LOGO)

	// MySQL 参数
	flag.StringVar(&USER_NAME, "user", "", "数据库用户名")
	flag.StringVar(&USER_PASSWD, "pass", "", "数据库密码")
	flag.StringVar(&ADDR, "addr", "localhost", "数据库地址")
	flag.StringVar(&PORT, "port", "3306", "数据库端口")
	flag.StringVar(&DB_NAME, "db", "", "数据库名")
	flag.StringVar(&OUTPUT_FILE, "output", time.Now().Format("2006_01_02")+"_backup.sql", "输出文件名")
	flag.BoolVar(&UPLOAD_OSS_FLAG, "up", false, "是否上传至 OSS")

	// OSS 参数
	flag.StringVar(&OSS_ENDPOINT, "oss-endpoint", "", "OSS Endpoint")
	flag.StringVar(&OSS_AK, "oss-ak", "", "OSS AccessKey ID")
	flag.StringVar(&OSS_SK, "oss-sk", "", "OSS AccessKey Secret")
	flag.StringVar(&OSS_BUCKETNAME, "oss-bucket", "", "OSS Bucket 名称")

	flag.Parse()

	// 设置 CONF 对象
	CONF.Endpoint = OSS_ENDPOINT
	CONF.AK = OSS_AK
	CONF.SK = OSS_SK
	CONF.BucketName = OSS_BUCKETNAME
}

func main() {
	if USER_NAME == "" || USER_PASSWD == "" || DB_NAME == "" {
		fmt.Println("请填写数据库用户名、密码和数据库名称。")
		os.Exit(1)
	}

	if UPLOAD_OSS_FLAG {
		if CONF.Endpoint == "" || CONF.AK == "" || CONF.SK == "" || CONF.BucketName == "" {
			fmt.Println("已指定-up(上传到存储同),请确保指定了以下参数：")
			fmt.Println("  -oss-endpoint")
			fmt.Println("  -oss-ak")
			fmt.Println("  -oss-sk")
			fmt.Println("  -oss-bucket")
			os.Exit(1)
		}
	}

	DSN := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		USER_NAME,
		USER_PASSWD,
		ADDR,
		PORT,
		DB_NAME,
	)

	f, err := os.Create(OUTPUT_FILE)
	if err != nil {
		fmt.Println("无法创建输出文件:", err)
		os.Exit(1)
	}
	defer f.Close()

	err = mysqldump.Dump(
		DSN,
		mysqldump.WithDropTable(),
		mysqldump.WithData(),
		mysqldump.WithTables(),
		mysqldump.WithWriter(f),
	)
	if err != nil {
		fmt.Println("导出数据库失败:", err)
		os.Exit(1)
	}

	fmt.Printf("数据库导出成功：%s\n", OUTPUT_FILE)

	info, err := f.Stat()
	if err != nil {
		fmt.Println("无法获取文件信息:", err)
		os.Exit(1)
	}
	fmt.Printf("文件大小：%s\n", formatBytes(info.Size()))

	if UPLOAD_OSS_FLAG {
		downloadURL, err := UploadFile(OUTPUT_FILE)
		if err != nil {
			fmt.Println("上传 OSS 失败:", err)
			os.Exit(2)
		}
		fmt.Println("上传成功，下载链接：", downloadURL)
	}
}
