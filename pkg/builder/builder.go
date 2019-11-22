package builder

import (
	"os"

	"smscp.xyz/internal/api"
	"smscp.xyz/internal/csv"
	"smscp.xyz/internal/db"
	"smscp.xyz/internal/db/fs"
	"smscp.xyz/internal/security"
	"smscp.xyz/internal/sms/twilio"
	"smscp.xyz/pkg/mode"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

var databaseConn, databaseErr = db.ConnDefault()

func Build(m mode.Mode) (*gin.Engine, error) {
	if databaseErr != nil {
		return nil, databaseErr
	}

	if m == mode.Test {
		gin.SetMode(gin.TestMode)
	}

	router := gin.Default()
	router.LoadHTMLGlob("web/html/*")
	router.Static("/static", "web/static/")
	store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	router.Use(sessions.Sessions(os.Getenv("SESSION_NAME"), store))

	security := security.Default(os.Getenv("JWT_SECRET"))

	data := db.Default(databaseConn, security, os.Getenv("MIGRATION_KEY"))
	data.SetMode(m)

	// For firestore rewrite, temp.
	data2 := fs.Default(os.Getenv("GOOGLE_PROJECT_ID"), security)

	sms := twilio.Default(os.Getenv("TWILIO_ID"), os.Getenv("TWILIO_SECRET"), os.Getenv("TWILIO_FROM") /* data */)

	csv := csv.Default()
	app := api.AppDefault(data, data2, sms, csv, security)

	router.GET("/", app.Page)
	router.POST("/", app.Page)

	router.GET("/ping", app.Pong)
	router.POST("/migrate", app.MigrateDB)

	router.POST("/user/login", app.UserLogin)
	router.POST("/user/create", app.UserCreate)
	router.POST("/user/update", app.UserUpdate)
	router.POST("/user/logout", app.UserLogout)

	// reset pass
	router.POST("/user/forgot-password", app.UserForgotPassword)
	router.GET("/reset/:hash", app.PageForgotPassword)
	router.POST("/reset/:hash", app.UserForgotPasswordNewPassword)

	router.POST("/note/create", app.NoteCreate)
	router.GET("/note/list/:page", app.NoteListJSON)

	router.POST("/cli/user/login", app.UserLoginCLI)
	router.POST("/cli/user/create", app.UserCreateCLI)
	router.POST("/cli/note/create", app.NoteCreateCLI)
	router.POST("/cli/note/latest", app.NoteLatestCLI)

	router.POST("/hook/sms/receive", app.HookSMS)

	// gdpr
	router.GET("/gdpr", app.UserExportAllData)
	router.POST("/gdpr", app.UserDeleteAllData)

	return router, nil
}
