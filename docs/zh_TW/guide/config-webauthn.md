# Webauthn

Webauthn 是一種用於安全身份驗證的網路標準。它允許使用者使用生物識別、行動裝置和 FIDO 安全金鑰登入網站。

Webauthn 是一種無密碼的身份驗證方法，提供了比傳統密碼更安全、易用的替代方案。

從 `v2.0.0-beta.34` 版本開始，Nginx UI 支援將 Webauthn Passkey 作為登入和雙因素認證（2FA）方法。

## Passkey

Passkey 是使用觸控、面部識別、裝置密碼或 PIN 驗證您身份的 Webauthn 憑證。它們可用作密碼替代品或作為 2FA 方法。

## 設定

為確保安全性，不能透過 UI 新增 Webauthn 設定。

請在 app.ini 設定檔中手動新增以下內容，並重新啟動 Nginx UI。

### RPDisplayName

- 類型：`string`

  用於在註冊新憑證時設定依賴方（RP）的顯示名稱。

### RPID

- 類型：`string`

  用於在註冊新憑證時設定依賴方（RP）的 ID。

### RPOrigins

- 類型：`[]string`

  用於在註冊新憑證時設定依賴方（RP）的來源（origins）。

完成後，重新整理此頁面並再次點選新增 Passkey。

由於某些瀏覽器的安全策略，除非在 `localhost` 上執行，否則無法在非 HTTPS 網站上使用 Passkey。

## 詳細說明

1. **使用 Passkey 的自動 2FA：**

   當您使用 Passkey 登入時，所有後續需要 2FA 的操作將自動使用 Passkey。這意味著您無需在 2FA 對話框中手動點選「透過 Passkey 進行認證」。

2. **刪除 Passkey：**

   如果您使用 Passkey 登入後，前往「設定 > 認證」並刪除目前的 Passkey，那麼在目前會話中，Passkey 將不再用於後續的 2FA 驗證。如果已設定基於時間的一次性密碼（TOTP），則將改為使用它；如果未設定，則將關閉 2FA。

3. **新增新 Passkey：**

   如果您在未使用 Passkey 的情況下登入，然後透過「設定 > 認證」新增新的 Passkey，那麼在目前會話中，新增的 Passkey 將優先用於後續所有的 2FA 驗證。
