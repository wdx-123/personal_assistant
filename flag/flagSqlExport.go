package flag

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"personal_assistant/global"
)

// SQLExport 导出 MySQL 数据
func SQLExport() error {
	mysql := global.Config.Mysql

	// 路径
	timer := time.Now().Format("20060102")
	sqlPath := fmt.Sprintf("mysql_%s.sql", timer)

	// 在docker容器中，执行导出命令
	//docker exec mysql mysqldump -u<username> -p<password> <db_name>
	// cmd 是一个未执行的命令对象，包含了导出数据库的完整指令，但尚未运行
	cmd := exec.Command("docker", "exec", "mysql", "mysqldump",
		"-u"+mysql.Username, "-p"+mysql.Password, mysql.DBName)

	// 创建文件
	outFile, err := os.Create(sqlPath)
	if err != nil {
		return err
	}

	// 将 mysqldump 的标准输出（stdout）重定向到 outFile
	cmd.Stdout = outFile
	defer func() {
		_ = outFile.Close()
	}()

	return cmd.Run()
}
