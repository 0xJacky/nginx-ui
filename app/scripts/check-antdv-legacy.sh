#!/bin/bash
# Scan for ant-design-vue APIs that antdv-next silently ignores or has renamed.
# Exits non-zero when anything is found. Run from the app/ directory.
set -u
found=0
check() { # $1 = label, $2 = grep -E pattern, $3.. = extra grep args
  local label="$1" pat="$2"; shift 2
  local hits
  hits=$(grep -rnE "$pat" src --include='*.vue' --include='*.ts' --include='*.tsx' "$@" 2>/dev/null)
  if [ -n "$hits" ]; then
    printf '\n[%s]\n%s\n' "$label" "$hits"
    found=1
  fi
}
check "leftover ant-design-vue import"      "from '@?ant-design(-vue)?|ant-design-vue/"
check "leftover @ant-design/icons-vue"      "@ant-design/icons-vue"
check "Drawer width (ignored, use :size)"   "<ADrawer[^>]*[^-]width="
check "Progress width/stroke-width"         "<AProgress[^>]*(stroke-)?width="
check "Spin tip (renders nothing)"          "<ASpin[^>]*[^-]tip="
check "Badge number-style (does not exist)" "number-style|numberStyle"
check "Dropdown #overlay (renders nothing)" "#overlay[^R]|v-slot:overlay"
check "overlay-class-name / overlay-style"  "overlay-class-name|overlayClassName|overlay-style|overlayStyle"
check "destroy-on-close"                    "destroy-on-close|destroyOnClose"
check "body-style / head-style"             "body-style=|bodyStyle:|head-style=|headStyle:"
check "value-style"                         "value-style=|valueStyle:"
check "Alert message prop"                  "<AAlert[^>]*[^-]message="
check "notification message key"            "notification\.[a-z]+\(\s*\{[^}]*message:"
check "AInputGroup"                         "<AInputGroup"
check "Input addon"                         "addon-before|addon-after|#addonBefore|#addonAfter"
# Select/Menu option children are exported by antdv-next but never forwarded to the
# underlying VcSelect/VcMenu, so they render an empty dropdown with no error anywhere.
check "Select option children (inert! use :options)" "<ASelectOption|<ASelectOptGroup"
check "Menu item children (inert! use :items)"       "<AMenuItem|<ASubMenu|<AMenuDivider"
check "Divider orientation (now means axis)"  "<ADivider[^>]*orientation=\"(left|right|center)\""
check "Typography copyable tooltip (is tooltips)" ":copyable=\"\\{[^\"]*tooltip:"
check "removed components"                  "<AList|<AListItem|<AComment|<AStep[^s]|<AIcon"
check ".ant-modal-content (not sizing box)" "\.ant-modal-content"
check "Form.useForm / validateInfos"        "Form\.useForm|validateInfos"
if [ "$found" = 0 ]; then echo "clean: no legacy antdv APIs found"; fi
exit $found
