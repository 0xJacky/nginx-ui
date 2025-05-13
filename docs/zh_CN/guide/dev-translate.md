# 翻译开发指南

## Weblate 在线翻译平台

我们很高兴地宣布 Nginx UI 的 Weblate 翻译平台现已进入公测阶段！这是我们通过多语言支持让 Nginx UI 面向全球用户的重要里程碑。

**快速开始：** 访问 [Weblate 平台](https://weblate.nginxui.com) 开始翻译工作。

### 关于 Weblate

Weblate 是一个功能强大且用户友好的翻译管理平台，它使社区成员能够高效地贡献翻译。该平台通过直观的界面简化了本地化流程，适合各种经验水平的贡献者使用。

### 如何参与贡献

我们欢迎所有对改善 Nginx UI 全球可访问性感兴趣的社区成员。无论您是母语使用者还是精通其他语言的用户，您的语言专长对项目都非常宝贵。

参与贡献的步骤：
1. 访问 [https://weblate.nginxui.com](https://weblate.nginxui.com)
2. 创建账户或使用 GitHub 登录
3. 选择您的目标语言
4. 开始翻译可用字符串

您的贡献将直接帮助扩大 Nginx UI 在全球的影响力。

### 支持与反馈

如果您对翻译平台有任何问题、疑问或改进建议，请通过 GitHub issues 或社区渠道提交反馈。

## 本地翻译环境

对于在本地进行翻译工作的开发者，我们推荐使用 i18n-gettext VSCode 扩展。

**扩展详情：**
- 文档：[GitHub 仓库](https://github.com/akinoccc/i18n-gettext)
- VSCode 应用商店：[i18n-gettext 扩展](https://marketplace.visualstudio.com/items?itemName=akino.i18n-gettext)

该扩展提供 AI 驱动的翻译功能，具有高质量输出，并支持额外的评分模型来验证翻译质量。

## 开发者翻译工作流

在进行影响可翻译内容的代码更改后，请运行以下命令更新翻译模板：

```bash
# 生成 Go i18n 文件
go generate

# 从前端提取可翻译字符串
cd app
pnpm gettext:extract
```

此过程确保所有新的可翻译内容都正确添加到翻译系统中。 