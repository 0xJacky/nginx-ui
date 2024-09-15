package user

import (
    "bytes"
    "crypto/sha1"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "github.com/0xJacky/Nginx-UI/api"
    "github.com/0xJacky/Nginx-UI/internal/crypto"
    "github.com/0xJacky/Nginx-UI/internal/user"
    "github.com/0xJacky/Nginx-UI/model"
    "github.com/0xJacky/Nginx-UI/query"
    "github.com/0xJacky/Nginx-UI/settings"
    "github.com/gin-gonic/gin"
    "github.com/pquerna/otp"
    "github.com/pquerna/otp/totp"
    "image/jpeg"
    "net/http"
    "strings"
)

func GenerateTOTP(c *gin.Context) {
    u := api.CurrentUser(c)

    issuer := fmt.Sprintf("Nginx UI %s", settings.ServerSettings.Name)
    issuer = strings.TrimSpace(issuer)

    otpOpts := totp.GenerateOpts{
        Issuer:      issuer,
        AccountName: u.Name,
        Period:      30, // seconds
        Digits:      otp.DigitsSix,
        Algorithm:   otp.AlgorithmSHA1,
    }
    otpKey, err := totp.Generate(otpOpts)
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    qrCode, err := otpKey.Image(512, 512)
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    // Encode the image to a buffer
    var buf []byte
    buffer := bytes.NewBuffer(buf)
    err = jpeg.Encode(buffer, qrCode, nil)
    if err != nil {
        fmt.Println("Error encoding image:", err)
        return
    }

    // Convert the buffer to a base64 string
    base64Str := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes())

    c.JSON(http.StatusOK, gin.H{
        "secret":  otpKey.Secret(),
        "qr_code": base64Str,
    })
}

func EnrollTOTP(c *gin.Context) {
    cUser := api.CurrentUser(c)
    if cUser.EnabledOTP() {
        c.JSON(http.StatusBadRequest, gin.H{
            "message": "User already enrolled",
        })
        return
    }

    if settings.ServerSettings.Demo {
        c.JSON(http.StatusBadRequest, gin.H{
            "message": "This feature is disabled in demo mode",
        })
        return
    }

    var json struct {
        Secret   string `json:"secret" binding:"required"`
        Passcode string `json:"passcode" binding:"required"`
    }
    if !api.BindAndValid(c, &json) {
        return
    }

    if ok := totp.Validate(json.Passcode, json.Secret); !ok {
        c.JSON(http.StatusNotAcceptable, gin.H{
            "message": "Invalid passcode",
        })
        return
    }

    ciphertext, err := crypto.AesEncrypt([]byte(json.Secret))
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    u := query.Auth
    _, err = u.Where(u.ID.Eq(cUser.ID)).Update(u.OTPSecret, ciphertext)
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    recoveryCode := sha1.Sum(ciphertext)

    c.JSON(http.StatusOK, gin.H{
        "message":       "ok",
        "recovery_code": hex.EncodeToString(recoveryCode[:]),
    })
}

func ResetOTP(c *gin.Context) {
    var json struct {
        RecoveryCode string `json:"recovery_code"`
    }
    if !api.BindAndValid(c, &json) {
        return
    }
    recoverCode, err := hex.DecodeString(json.RecoveryCode)
    if err != nil {
        api.ErrHandler(c, err)
        return
    }
    cUser := api.CurrentUser(c)
    k := sha1.Sum(cUser.OTPSecret)
    if !bytes.Equal(k[:], recoverCode) {
        c.JSON(http.StatusBadRequest, gin.H{
            "message": "Invalid recovery code",
        })
        return
    }

    u := query.Auth
    _, err = u.Where(u.ID.Eq(cUser.ID)).UpdateSimple(u.OTPSecret.Null())
    if err != nil {
        api.ErrHandler(c, err)
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "message": "ok",
    })
}

func OTPStatus(c *gin.Context) {
    status := false
    u, ok := c.Get("user")
    if ok {
        status = u.(*model.Auth).EnabledOTP()
    }
    c.JSON(http.StatusOK, gin.H{
        "status": status,
    })
}

func SecureSessionStatus(c *gin.Context) {
    u, ok := c.Get("user")
    if !ok || !u.(*model.Auth).EnabledOTP() {
        c.JSON(http.StatusOK, gin.H{
            "status": false,
        })
        return
    }
    ssid := c.GetHeader("X-Secure-Session-ID")
    if ssid == "" {
        ssid = c.Query("X-Secure-Session-ID")
    }
    if ssid == "" {
        c.JSON(http.StatusOK, gin.H{
            "status": false,
        })
        return
    }

    if user.VerifySecureSessionID(ssid, u.(*model.Auth).ID) {
        c.JSON(http.StatusOK, gin.H{
            "status": true,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status": false,
    })
}

func StartSecure2FASession(c *gin.Context) {
    var json struct {
        OTP          string `json:"otp"`
        RecoveryCode string `json:"recovery_code"`
    }
    if !api.BindAndValid(c, &json) {
        return
    }
    u := api.CurrentUser(c)
    if !u.EnabledOTP() {
        c.JSON(http.StatusBadRequest, gin.H{
            "message": "User not configured with 2FA",
        })
        return
    }

    if json.OTP == "" && json.RecoveryCode == "" {
        c.JSON(http.StatusBadRequest, LoginResponse{
            Message: "The user has enabled 2FA",
        })
        return
    }

    if err := user.VerifyOTP(u, json.OTP, json.RecoveryCode); err != nil {
        c.JSON(http.StatusBadRequest, LoginResponse{
            Message: "Invalid 2FA or recovery code",
        })
        return
    }

    sessionId := user.SetSecureSessionID(u.ID)

    c.JSON(http.StatusOK, gin.H{
        "session_id": sessionId,
    })
}
