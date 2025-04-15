# Logrotate

在這個部分，我們將介紹 Nginx UI 中關於 logrotate 的設定選項。

**logrotate** 目的是簡化生成大量日誌文件的系統的管理。
它可以按天、周、月或者文件大小來輪轉日誌文件，還可以壓縮、刪除舊的日誌文件，以及傳送日誌文件到指定的郵箱。
預設情況下，對於在主機上安裝 Nginx UI 的使用者，大多數主流的 Linux 發行版都已整合 logrotate，
所以你不需要修改任何東西。

對於使用 Docker 容器安裝 Nginx UI 的使用者，你可以手動啟用這個選項。
Nginx UI 的 crontab 任務排程器將會按照你設定的分鐘間隔執行 logrotate 命令。

## Enabled
- 類型：`bool`
- 預設值：`false`

這個選項用於在 Nginx UI 中啟用 logrotate crontab 任務。

## CMD
- 類型：`string`
- 預設值：`logrotate /etc/logrotate.d/nginx`

這個選項用於在 Nginx UI 中設定 logrotate 命令。

## Interval
- 類型：`int`
- 預設值：`1440`

這個選項用於在 Nginx UI 中設定 logrotate crontab 任務的分鐘間隔。
