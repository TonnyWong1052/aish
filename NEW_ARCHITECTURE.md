# AISH TUI 設定架構圖

```mermaid
sequenceDiagram
    actor User
    participant TUI as "Settings TUI<br>(settings_tui.go)"
    participant Def as "Settings Definition<br>(settings_definition.go)"
    participant CfgFile as "Config File<br>(~/.config/aish/config.json)"
    participant ShellMod as "Shell Module<br>(internal/shell/hook.go)"

    User->>+TUI: 執行 `aish config`
    TUI->>+CfgFile: 載入 `config.json`
    CfgFile-->>-TUI: 回傳 Config 物件
    
    TUI->>+Def: GetSettingsDefinition(Config)
    Def->>+ShellMod: 檢查 Hook 安裝狀態
    ShellMod-->>-Def: 回傳狀態 (Installed/Not Installed)
    Def-->>-TUI: 回傳完整的 `[]SettingItem` (包含 UI 元數據和 Hook 狀態)

    TUI-->>User: 顯示互動式設定列表

    alt 使用者修改設定 (例如：切換布林值)
        User->>TUI: 按下 Enter 鍵
        TUI->>TUI: 更新內部的 Config 副本
        TUI-->>User: 重新繪製畫面，顯示新值
    end

    alt 使用者觸發動作 (例如：安裝 Hook)
        User->>TUI: 在 "Install Hook" 項目上按下 Enter
        TUI->>Def: 取得對應的 `Action` 函式
        TUI->>+ShellMod: 執行 `InstallHook()`
        ShellMod-->>-TUI: 回傳成功/失敗
        
        TUI->>+Def: 再次呼叫 GetSettingsDefinition() 以更新狀態
        Def->>+ShellMod: 再次檢查 Hook 狀態
        ShellMod-->>-Def: 回傳新狀態 (Installed)
        Def-->>-TUI: 回傳更新後的 `[]SettingItem`
        TUI-->>User: 重新繪製畫面 (按鈕變為 "Uninstall Hook")
    end

    User->>TUI: 按下 's' 儲存並退出
    TUI->>+CfgFile: 將更新後的 Config 物件寫入 `config.json`
    CfgFile-->>-TUI: 回傳成功
    TUI-->>-User: 顯示成功訊息並結束程式
```

### 圖解說明

1.  **啟動與載入**：使用者執行 `aish config`。TUI 啟動後，首先從磁碟載入 `config.json`。
2.  **動態定義**：TUI 接著呼叫 `GetSettingsDefinition`。此函式會先向 `Shell Module` 查詢 `Hook` 的安裝狀態，然後結合 `Config` 物件和 `Hook` 狀態，動態生成一份包含所有 UI 元數據的設定列表 (`[]SettingItem`)。
3.  **渲染**：TUI 根據這份定義列表，渲染出使用者看到的互動介面。
4.  **互動：修改值**：當使用者修改一個普通的設定（如開關），TUI 只會更新其內部的 `Config` 副本，並立即重繪螢幕以反映變更。這個過程不涉及讀寫檔案或呼叫外部模組。
5.  **互動：執行動作**：當使用者觸發一個動作（如安裝 `Hook`），TUI 會執行 `SettingItem` 中定義的 `Action` 函式，直接呼叫 `Shell Module`。動作完成後，TUI 會**重新**執行第 2 步（動態定義），以獲取最新的狀態（例如 `Hook` 已安裝），並刷新整個介面。
6.  **儲存**：當使用者選擇儲存時，TUI 會將其內部持有的、已更新的 `Config` 副本一次性寫入 `config.json`。

這種設計將「狀態查詢」和「動作執行」與 UI 渲染清晰地分開，並透過元數據定義進行解耦，實現了高度的靈活性和可擴展性。