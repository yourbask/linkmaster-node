package recovery

import (
	"runtime/debug"

	"go.uber.org/zap"
)

var logger *zap.Logger

func Init() {
	// 初始化logger（这里简化处理，实际应该从外部传入）
	logger, _ = zap.NewProduction()
}

// Recover 恢复panic
func Recover() {
	if r := recover(); r != nil {
		logger.Error("发生panic",
			zap.Any("panic", r),
			zap.String("stack", string(debug.Stack())),
		)
	}
}

