# 翻譯開發指南

## Weblate 線上翻譯平臺

我們很高興地宣布 Nginx UI 的 Weblate 翻譯平臺現已進入公測階段！這是我們透過多語言支援讓 Nginx UI 面向全球用戶的重要里程碑。

**快速開始：** 訪問 [Weblate 平臺](https://weblate.nginxui.com) 開始翻譯工作。

### 關於 Weblate

Weblate 是一個功能強大且用戶友好的翻譯管理平臺，它使社區成員能夠高效地貢獻翻譯。該平臺透過直觀的界面簡化了本地化流程，適合各種經驗水平的貢獻者使用。

### 如何參與貢獻

我們歡迎所有對改善 Nginx UI 全球可訪問性感興趣的社區成員。無論您是母語使用者還是精通其他語言的用戶，您的語言專長對專案都非常寶貴。

參與貢獻的步驟：
1. 訪問 [https://weblate.nginxui.com](https://weblate.nginxui.com)
2. 創建帳戶或使用 GitHub 登入
3. 選擇您的目標語言
4. 開始翻譯可用字符串

您的貢獻將直接幫助擴大 Nginx UI 在全球的影響力。

### 支援與反饋

如果您對翻譯平臺有任何問題、疑問或改進建議，請透過 GitHub issues 或社區渠道提交反饋。

## 本地翻譯環境

對於在本地進行翻譯工作的開發者，我們推薦使用 i18n-gettext VSCode 擴展。

**擴展詳情：**
- 文檔：[GitHub 倉庫](https://github.com/akinoccc/i18n-gettext)
- VSCode 應用商店：[i18n-gettext 擴展](https://marketplace.visualstudio.com/items?itemName=akino.i18n-gettext)

該擴展提供 AI 驅動的翻譯功能，具有高品質輸出，並支援額外的評分模型來驗證翻譯品質。

## 開發者翻譯工作流

在進行影響可翻譯內容的代碼更改後，請運行以下命令更新翻譯模板：

```bash
# 生成 Go i18n 文件
go generate

# 從前端提取可翻譯字符串
cd app
pnpm gettext:extract
```

此過程確保所有新的可翻譯內容都正確添加到翻譯系統中。 