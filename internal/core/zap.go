package core

import (
	"log"
	"os"

	"personal_assistant/global"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	// 解决先有鸡，还是先有蛋的问题，初期config未配置的时候，进行简单的配置
	global.Log, _ = zap.NewDevelopment()
	// 后期会通过，第二阶段优化
}

// InitLogger 初始化并返回一个基于配置设置的新 zap.Logger 实例
func InitLogger() *zap.Logger {
	zapCfg := global.Config.Zap
	// 创建一个用于日志输出的 writeSyncer
	writeSyncer := getLogWriter(zapCfg.Filename, zapCfg.MaxSize, zapCfg.MaxBackups, zapCfg.MaxAge)

	// 如果配置了控制台输出，则添加控制台输出
	if zapCfg.IsConsolePrint {
		writeSyncer = zapcore.NewMultiWriteSyncer(writeSyncer, zapcore.AddSync(os.Stdout))
	}
	// 创建日志格式化的编码器
	encoder := getEncoder()

	// 根据配置确定日志级别
	var logLevel zapcore.Level
	if err := logLevel.UnmarshalText([]byte(zapCfg.Level)); err != nil {
		log.Fatalf("Failed to parse log level: %v", err)
	}

	// 创建核心和日志实例
	core := zapcore.NewCore(encoder, writeSyncer, logLevel)
	logger := zap.New(core, zap.AddCaller())
	return logger
}

// getLogWriter 返回一个 zapcore.WriteSyncer，该写入器。利用 lumberjack 包，实现日志的滚动记录
func getLogWriter(filename string, maxSize, maxBackups, maxAge int) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   filename,   // 日志文件的位置
		MaxSize:    maxSize,    // 在进行切割之前，日志文件的最大大小（以MB为单位）
		MaxBackups: maxBackups, // 保留旧文件的最大个数
		MaxAge:     maxAge,     // 保留旧文件的最大天数
	}
	return zapcore.AddSync(lumberJackLogger)
}

// getEncoder 返回一个生产日志配置的 JSON 编码器
func getEncoder() zapcore.Encoder {
	// 1. 获取生产环境默认配置
	encoderConfig := zap.NewProductionEncoderConfig()

	// 在细节地方进行修正
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder         // 设置日志时间的编码格式为ISO8601标准（如2024-09-17T15:30:00+08:00）
	encoderConfig.TimeKey = "time"                                // 指定日志中时间字段的键名为"time"（日志输出时时间会以该键存储）
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder       // 设置日志级别（如INFO、ERROR）的编码格式为大写形式
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder // 设置日志中持续时间的编码格式为秒级（如将1500ms转换为1.5s）
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder       // 设置日志调用者信息（文件路径和行号）的编码格式为简短形式（如缩短文件绝对路径为相对路径）

	return zapcore.NewJSONEncoder(encoderConfig)
}

/*
	zapcore.ISO8601TimeEncoder 标准

	2024-09-17T15:30:45+08:00
	我们拆解开每部分的含义，就好懂了：
		2024-09-17：日期（年 - 月 - 日，固定用 - 分隔）；
	T：一个特殊符号，用来分隔 “日期” 和 “时间”（是标准规定的，不是字母 “T” 的意思）；
		15:30:45：时间（24 小时制，时：分: 秒，固定用 : 分隔）；
	+08:00：时区（表示 “比世界标准时间（UTC）快 8 小时”，也就是我们常用的北京时间）。

*/

// github.com/natefinch/lumberjack 是 Go 语言中一个专门用于 日志文件轮转（log rotation） 的第三方库，
// 主要解决「日志文件无限增长导致磁盘占满」的问题。

// 在 Zap 日志库中，zap.AddCaller() 是一个 日志配置选项，作用是 在日志中自动添加「调用者信息」—— 即输出这条日志的代码所在的 文件名和行号。
// {"level":"info","time":"2024-09-18T10:00:00+08:00","caller":"main.go:30","message":"用户登录成功"}
// 中的，main.go:30。
// 如果不添加，则没有
