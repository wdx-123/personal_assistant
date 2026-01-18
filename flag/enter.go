package flag

import (
	"errors"
	"fmt"

	"os"
	"personal_assistant/global"

	"github.com/urfave/cli"
	"go.uber.org/zap"
) // 是一个 Go 语言的包，专门用于构建命令行工具（Command Line Interface, CLI）。

// 一套代码，既能当管理工具用，又能当主程序用
// 这段代码是在程序启动时检查命令行参数的「分流器」，决定程序是执行管理任务还是运行主逻辑。
/*
	方法	                用于什么标志  返回值	        示例	                     相当于问
	c.Bool("flag名")	BoolFlag	true 或 false	c.Bool("sql")	        「用户打开这个开关了吗？」
	c.IsSet("flag名")	StringFlag	true 或 false	c.IsSet("sql-import")	「用户设置了这个选项吗？」
	c.String("flag名")	StringFlag	具体的字符串值	c.String("sql-import")	「用户把这个选项设置成什么值了？」

	并且
	c.Bool 不需要值，标志本身即表示 true。
	c.IsSet 不关心值，仅检测标志是否出现。
*/
var (
	sqlFlag = &cli.BoolFlag{
		Name:  "sql",
		Usage: "Initializes the structure of the MySQL entity table.",
	}
	sqlExportFlag = &cli.BoolFlag{
		Name:  "sql-export",
		Usage: "Exports SQL data to a specified file.",
	}
	sqlImportFlag = &cli.StringFlag{
		Name:  "sql-import",
		Usage: "Imports SQL data from a specified file.",
	}
	adminFlag = &cli.BoolFlag{
		Name:  "admin",
		Usage: "Creates an administrator using the name, email and address specified in the configs.yaml file.",
	}
)

// Run 执行基于命令行标志的相应操作
// 它处理不同的标志，执行相应操作，并记录成功或错误的消息
func Run(c *cli.Context) {
	// 检查是否设置了多个标志
	if c.NumFlags() > 1 {
		err := cli.NewExitError("Only one command can be specified", 1)
		global.Log.Error("Invalid command usage:", zap.Error(err))
		os.Exit(1)
	}

	// 根据不同的标志选择执行的操作
	switch {
	case c.Bool(sqlFlag.Name):
		if err := SQL(); err != nil {
			global.Log.Error("Failed to create table structure:", zap.Error(err))
			return
		} else {
			global.Log.Info("Successfully created table structure")
		}
	case c.Bool(sqlExportFlag.Name):
		if err := SQLExport(); err != nil {
			global.Log.Error("Failed to export SQL data:", zap.Error(err))
		} else {
			global.Log.Info("Successfully exported SQL data")
		}
	case c.IsSet(sqlImportFlag.Name):
		if errs := SQLImport(c.String(sqlImportFlag.Name)); len(errs) > 0 {
			var combinedErrors string
			for _, err := range errs {
				combinedErrors += err.Error() + "\n"
			}
			err := errors.New(combinedErrors)
			global.Log.Error("Failed to import SQL data:", zap.Error(err))
		}
	default:
		err := cli.NewExitError("unknown command", 1)
		global.Log.Error(err.Error(), zap.Error(err))
	}

}

// NewApp 创建并配置一个新的 CLI 应用程序，设置标志和默认操作
func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = "Go Blog"

	// 这段代码是 CLI应用程序的标志注册部分 ，它定义了Go博客系统支持的所有命令行参数。
	app.Flags = []cli.Flag{
		sqlFlag,       // --sql
		sqlExportFlag, // --sql-export
		sqlImportFlag, // --sql-import
		adminFlag,     // --admin
	}
	app.Action = Run
	return app
}

// InitFlag 初始化并运行 CLI 应用程序
func InitFlag() {
	if len(os.Args) > 1 {
		app := NewApp()
		err := app.Run(os.Args)
		if err != nil {
			global.Log.Error("Application execution encountered an error:", zap.Error(err))
			os.Exit(1)
		}
		if os.Args[1] == "-h" || os.Args[1] == "-help" {
			fmt.Println("Displaying help message...")
		}
		os.Exit(0)
	}
}

/*
	多种情况，解析
	./blog-service			运行函数
	./blog-service -h 		无效
	./blog-service --sql 	执行
	./blog-service --invalid 无效

*/
