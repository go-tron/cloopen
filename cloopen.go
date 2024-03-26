package cloopen

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/go-playground/validator/v10"
	baseError "github.com/go-tron/base-error"
	"github.com/go-tron/config"
	localTime "github.com/go-tron/local-time"
	"github.com/go-tron/logger"
	"github.com/parnurzeal/gorequest"
	"strings"
)

var (
	ErrorParam    = baseError.SystemFactory("3001", "验证码服务参数错误:{}")
	ErrorTemplate = baseError.SystemFactory("3002", "验证码服务模板错误:{}")
	ErrorRequest  = baseError.SystemFactory("3003", "验证码服务连接失败:{}")
	ErrorResponse = baseError.SystemFactory("3004", "验证码服务返回失败:{}")
	ErrorFail     = baseError.SystemFactory("3005")
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

func New(config *Cloopen) *Cloopen {
	if config.AccountSid == "" || config.AccountToken == "" || config.AppId == "" || config.Logger == nil {
		panic("invalid cloopen config")
	}
	return config
}

func NewWithConfig(c *config.Config) *Cloopen {
	return New(&Cloopen{
		ServerIP:     "app.cloopen.com",
		ServerPort:   "8883",
		SoftVersion:  "2013-12-26",
		AccountSid:   c.GetString("cloopen.accountSid"),
		AccountToken: c.GetString("cloopen.accountToken"),
		AppId:        c.GetString("cloopen.appId"),
		DisplayNum:   c.GetString("cloopen.displayNum"),
		PlayTimes:    c.GetString("cloopen.playTimes"),
		MaxCallTime:  c.GetString("cloopen.maxCallTime"),
		Logger:       logger.NewZapWithConfig(c, "cloopen", "info"),
		Templates:    c.GetStringMapString("cloopen.templates"),
	})
}

type Cloopen struct {
	Logger       logger.Logger
	AccountSid   string
	AccountToken string
	AppId        string
	ServerIP     string
	ServerPort   string
	SoftVersion  string
	DisplayNum   string
	PlayTimes    string
	MaxCallTime  string
	Templates    map[string]string
}

type TextOption struct {
	TemplateId   string   `json:"templateId"`
	TemplateName string   `json:"templateName" validate:"required"`
	To           string   `json:"to" validate:"required"`
	Datas        []string `json:"datas" validate:"required"`
}

func (c *Cloopen) Text(option *TextOption) (err error) {

	var (
		response = ""
		errs     []error
	)
	defer func() {
		c.Logger.Info("",
			c.Logger.Field("phone", option.To),
			c.Logger.Field("templateName", option.TemplateName),
			c.Logger.Field("templateId", option.TemplateId),
			c.Logger.Field("error", err),
			c.Logger.Field("response", response),
		)
	}()

	if err := validate.Struct(option); err != nil {
		return ErrorParam(err)
	}

	templateId := option.TemplateId
	if templateId == "" {
		templateId = c.Templates[option.TemplateName]
	}

	if templateId == "" {
		return ErrorTemplate()
	}

	var data = make(map[string]interface{})
	data["appId"] = c.AppId
	data["templateId"] = templateId
	data["to"] = option.To
	data["datas"] = option.Datas

	authStr := c.AccountSid + ":" + localTime.Now().Compact()

	auth := base64.StdEncoding.EncodeToString([]byte(authStr))

	hash := md5.New()
	sigStr := c.AccountSid + c.AccountToken + localTime.Now().Compact()
	hash.Write([]byte(sigStr))
	sig := hex.EncodeToString(hash.Sum(nil))
	sig = strings.ToUpper(sig)

	url := "https://" + c.ServerIP + ":" + c.ServerPort + "/" + c.SoftVersion + "/Accounts/" + c.AccountSid + "/SMS/TemplateSMS?sig=" + sig

	_, response, errs = gorequest.New().Post(url).
		Set("Accept", "application/json").
		Set("Authorization", auth).
		Send(data).
		End()

	if errs != nil {
		return ErrorRequest(errs)
	}

	var bodyMap map[string]interface{}

	if err := json.Unmarshal([]byte(response), &bodyMap); err != nil {
		return ErrorResponse(err)
	}

	if bodyMap["statusCode"] != "000000" {
		var errorMsg = "发送失败"
		if bodyMap["statusMsg"] != nil {
			errorMsg = bodyMap["statusMsg"].(string)
		}
		return ErrorFail(errorMsg)
	}

	return nil
}
