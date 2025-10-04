package prompt

import (
	"encoding/json"
	"fmt"
	"os"
)

// Manager handles loading and accessing prompts.
type Manager struct {
	prompts map[string]map[string]string
}

// NewManager creates a prompt manager from a file.
func NewManager(path string) (*Manager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var prompts map[string]map[string]string
	if err := json.Unmarshal(data, &prompts); err != nil {
		return nil, err
	}

	return &Manager{prompts: prompts}, nil
}

// NewDefaultManager creates a prompt manager with built-in default prompts.
func NewDefaultManager() *Manager {
	defaultPrompts := map[string]map[string]string{
		"generate_command": {
			"en": "You are a shell command generator for macOS. Output ONLY a single-line JSON object with the exact schema: {\"command\":\"<shell>\"}. No prose, no markdown, no extra keys. Use a safe, single command. The command MUST be a valid macOS shell command. If the prompt is a general question or cannot be performed, return an echo command that prints a concise answer, e.g., {\"command\":\"echo '...simple answer...'\"}. The command should be directly usable, not like `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
            "zh-TW":      "你是 macOS 的指令產生器。僅輸出一行 JSON，結構嚴格為：{\"command\":\"<shell>\"}。不要輸出說明、Markdown 或多餘鍵。必須輸出有效的 macOS Shell 指令。若使用者的提示屬一般問答或無法執行，請輸出 echo 指令將簡短答案印出，例如：{\"command\":\"echo '...簡短答案...'\"}。指令需可直接使用，避免產生如 `ls -a \"<path_to_directory_or_file>\"` 的佔位符。\n提示：{{.Prompt}}\nJSON：",
			"zh-CN":      "你是 macOS 的命令生成器。只输出一行 JSON，结构严格为：{\"command\":\"<shell>\"}。不要输出说明、Markdown 或多余键。请生成安全且可执行的单一命令，命令需可直接使用，避免生成如 `ls -a \"<path_to_directory_or_file>\"` 的占位符。\n提示：{{.Prompt}}\nJSON：",
			"japanese":   "あなたは macOS のシェルコマンド生成器です。正確なスキーマ {\"command\":\"<shell>\"} で単一行の JSON オブジェクトのみを出力してください。散文、Markdown、余分なキーは含めないでください。安全で単一のコマンドを使用してください。コマンドは直接使用可能である必要があり、`ls -a \"<path_to_directory_or_file>\"` のようなプレースホルダーを生成しないでください。\nプロンプト：{{.Prompt}}\nJSON：",
			"korean":     "당신은 macOS용 셸 명령어 생성기입니다. 정확한 스키마 {\"command\":\"<shell>\"}로 단일 라인 JSON 객체만 출력하세요. 산문, 마크다운, 추가 키는 포함하지 마세요. 안전하고 단일 명령어를 사용하세요. 명령어는 직접 사용 가능해야 하며, `ls -a \"<path_to_directory_or_file>\"`와 같은 플레이스홀더를 생성하지 마세요.\n프롬프트：{{.Prompt}}\nJSON：",
			"spanish":    "Eres un generador de comandos de shell para macOS. Solo emite un objeto JSON de una línea con el esquema exacto: {\"command\":\"<shell>\"}. Sin prosa, sin markdown, sin claves extra. Usa un comando seguro y único. El comando debe ser directamente utilizable, no como `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"french":     "Vous êtes un générateur de commandes shell pour macOS. Ne sortez qu'un objet JSON d'une ligne avec le schéma exact : {\"command\":\"<shell>\"}. Pas de prose, pas de markdown, pas de clés supplémentaires. Utilisez une commande sûre et unique. La commande doit être directement utilisable, pas comme `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"german":     "Sie sind ein Shell-Befehl-Generator für macOS. Geben Sie nur ein einzeiliges JSON-Objekt mit dem exakten Schema aus: {\"command\":\"<shell>\"}. Keine Prosa, kein Markdown, keine zusätzlichen Schlüssel. Verwenden Sie einen sicheren, einzelnen Befehl. Der Befehl sollte direkt verwendbar sein, nicht wie `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"italian":    "Sei un generatore di comandi shell per macOS. Emetti solo un oggetto JSON a riga singola con lo schema esatto: {\"command\":\"<shell>\"}. Niente prosa, niente markdown, niente chiavi extra. Usa un comando sicuro e singolo. Il comando dovrebbe essere direttamente utilizzabile, non come `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"portuguese": "Você é um gerador de comandos shell para macOS. Emita apenas um objeto JSON de linha única com o esquema exato: {\"command\":\"<shell>\"}. Sem prosa, sem markdown, sem chaves extras. Use um comando seguro e único. O comando deve ser diretamente utilizável, não como `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"russian":    "Вы генератор команд оболочки для macOS. Выводите только однострочный JSON объект с точной схемой: {\"command\":\"<shell>\"}. Без прозы, без markdown, без лишних ключей. Используйте безопасную, единственную команду. Команда должна быть непосредственно применимой, не как `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
			"arabic":     "أنت مولد أوامر shell لـ macOS. أخرج فقط كائن JSON بسطر واحد بالمخطط الدقيق: {\"command\":\"<shell>\"}. بدون نثر، بدون markdown، بدون مفاتيح إضافية. استخدم أمرًا آمنًا واحدًا. يجب أن يكون الأمر قابلاً للاستخدام مباشرة، وليس مثل `ls -a \"<path_to_directory_or_file>\"`.\nPrompt: {{.Prompt}}\nJSON:",
		},
		"get_suggestion": {
			"en":         "You are a shell debugging assistant on macOS. Output ONLY one JSON object with schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Do not include markdown or extra keys.\nCommand: {{.Command}}\nExit Code: {{.ExitCode}}\nStdout:\n{{.Stdout}}\nStderr:\n{{.Stderr}}\nJSON:",
			"zh-TW":      "你是 macOS 的指令除錯助理。僅輸出一個 JSON 物件，結構嚴格為：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多餘鍵。\n指令：{{.Command}}\n結束代碼：{{.ExitCode}}\n標準輸出：\n{{.Stdout}}\n標準錯誤：\n{{.Stderr}}\nJSON：",
			"zh-CN":      "你是 macOS 的命令调试助手。只输出一个 JSON 对象，结构严格为：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多余键。\n命令：{{.Command}}\n退出代码：{{.ExitCode}}\n标准输出：\n{{.Stdout}}\n标准错误：\n{{.Stderr}}\nJSON：",
			"japanese":   "あなたは macOS のシェルデバッグアシスタントです。スキーマ {\"explanation\":\"...\",\"command\":\"<shell>\"} で JSON オブジェクトを一つだけ出力してください。Markdown や余分なキーは含めないでください。\nコマンド：{{.Command}}\n終了コード：{{.ExitCode}}\n標準出力：\n{{.Stdout}}\n標準エラー：\n{{.Stderr}}\nJSON：",
			"korean":     "당신은 macOS용 셸 디버깅 어시스턴트입니다. 스키마 {\"explanation\":\"...\",\"command\":\"<shell>\"}로 JSON 객체를 하나만 출력하세요. 마크다운이나 추가 키는 포함하지 마세요.\n명령어：{{.Command}}\n종료 코드：{{.ExitCode}}\n표준 출력：\n{{.Stdout}}\n표준 오류：\n{{.Stderr}}\nJSON：",
			"spanish":    "Eres un asistente de depuración de shell en macOS. Solo emite un objeto JSON con esquema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. No incluyas markdown o claves extra.\nComando: {{.Command}}\nCódigo de Salida: {{.ExitCode}}\nSalida Estándar:\n{{.Stdout}}\nError Estándar:\n{{.Stderr}}\nJSON:",
			"french":     "Vous êtes un assistant de débogage shell sur macOS. Ne sortez qu'un objet JSON avec le schéma : {\"explanation\":\"...\",\"command\":\"<shell>\"}. N'incluez pas de markdown ou de clés supplémentaires.\nCommande: {{.Command}}\nCode de Sortie: {{.ExitCode}}\nSortie Standard:\n{{.Stdout}}\nErreur Standard:\n{{.Stderr}}\nJSON:",
			"german":     "Sie sind ein Shell-Debugging-Assistent auf macOS. Geben Sie nur ein JSON-Objekt mit Schema aus: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Fügen Sie kein Markdown oder zusätzliche Schlüssel hinzu.\nBefehl: {{.Command}}\nExit-Code: {{.ExitCode}}\nStandardausgabe:\n{{.Stdout}}\nStandardfehler:\n{{.Stderr}}\nJSON:",
			"italian":    "Sei un assistente di debug shell su macOS. Emetti solo un oggetto JSON con schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Non includere markdown o chiavi extra.\nComando: {{.Command}}\nCodice di Uscita: {{.ExitCode}}\nOutput Standard:\n{{.Stdout}}\nErrore Standard:\n{{.Stderr}}\nJSON:",
			"portuguese": "Você é um assistente de depuração shell no macOS. Emita apenas um objeto JSON com esquema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Não inclua markdown ou chaves extras.\nComando: {{.Command}}\nCódigo de Saída: {{.ExitCode}}\nSaída Padrão:\n{{.Stdout}}\nErro Padrão:\n{{.Stderr}}\nJSON:",
			"russian":    "Вы помощник по отладке оболочки на macOS. Выводите только один JSON объект со схемой: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Не включайте markdown или лишние ключи.\nКоманда: {{.Command}}\nКод выхода: {{.ExitCode}}\nСтандартный вывод:\n{{.Stdout}}\nСтандартная ошибка:\n{{.Stderr}}\nJSON:",
			"arabic":     "أنت مساعد تصحيح أخطاء shell على macOS. أخرج فقط كائن JSON واحد بالمخطط: {\"explanation\":\"...\",\"command\":\"<shell>\"}. لا تتضمن markdown أو مفاتيح إضافية.\nالأمر: {{.Command}}\nرمز الخروج: {{.ExitCode}}\nالإخراج القياسي:\n{{.Stdout}}\nخطأ قياسي:\n{{.Stderr}}\nJSON:",
		},
		"get_enhanced_suggestion": {
			"en":         "You are a shell debugging assistant on macOS with enhanced context awareness. Output ONLY one JSON object with schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Do not include markdown or extra keys.\n\nFailed Command: {{.Command}}\nExit Code: {{.ExitCode}}\nStdout:\n{{.Stdout}}\nStderr:\n{{.Stderr}}\n\nContext Information:\nWorking Directory: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Recent Command History:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Directory Contents:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"zh-TW":      "你是具備進階上下文感知的 macOS 指令除錯助理。僅輸出一個 JSON 物件，結構嚴格為：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多餘鍵。\n\n失敗指令：{{.Command}}\n結束代碼：{{.ExitCode}}\n標準輸出：\n{{.Stdout}}\n標準錯誤：\n{{.Stderr}}\n\n上下文資訊：\n工作目錄：{{.WorkingDirectory}}\n終端類型：{{.ShellType}}\n\n{{if .RecentCommands}}最近指令歷史：\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}目錄內容：\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON：",
			"zh-CN":      "你是具备高级上下文感知的 macOS 命令调试助手。只输出一个 JSON 对象，结构严格为：{\"explanation\":\"...\",\"command\":\"<shell>\"}。不要包含 Markdown 或多余键。\n\n失败命令：{{.Command}}\n退出代码：{{.ExitCode}}\n标准输出：\n{{.Stdout}}\n标准错误：\n{{.Stderr}}\n\n上下文信息：\n工作目录：{{.WorkingDirectory}}\n终端类型：{{.ShellType}}\n\n{{if .RecentCommands}}最近命令历史：\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}目录内容：\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON：",
			"japanese":   "あなたは高度なコンテキスト認識を備えた macOS のシェルデバッグアシスタントです。スキーマ {\"explanation\":\"...\",\"command\":\"<shell>\"} で JSON オブジェクトを一つだけ出力してください。Markdown や余分なキーは含めないでください。\n\n失敗したコマンド：{{.Command}}\n終了コード：{{.ExitCode}}\n標準出力：\n{{.Stdout}}\n標準エラー：\n{{.Stderr}}\n\nコンテキスト情報：\n作業ディレクトリ：{{.WorkingDirectory}}\nシェル：{{.ShellType}}\n\n{{if .RecentCommands}}最近のコマンド履歴：\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}ディレクトリ内容：\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON：",
			"korean":     "고급 컨텍스트 인식을 갖춘 macOS용 셸 디버깅 어시스턴트입니다. 스키마 {\"explanation\":\"...\",\"command\":\"<shell>\"}로 JSON 객체를 하나만 출력하세요. 마크다운이나 추가 키는 포함하지 마세요.\n\n실패한 명령어：{{.Command}}\n종료 코드：{{.ExitCode}}\n표준 출력：\n{{.Stdout}}\n표준 오류：\n{{.Stderr}}\n\n컨텍스트 정보：\n작업 디렉토리：{{.WorkingDirectory}}\n셸：{{.ShellType}}\n\n{{if .RecentCommands}}최근 명령어 기록：\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}디렉토리 내용：\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON：",
			"spanish":    "Eres un asistente de depuración de shell en macOS con conciencia de contexto mejorada. Solo emite un objeto JSON con esquema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. No incluyas markdown o claves extra.\n\nComando Fallido: {{.Command}}\nCódigo de Salida: {{.ExitCode}}\nSalida Estándar:\n{{.Stdout}}\nError Estándar:\n{{.Stderr}}\n\nInformación de Contexto:\nDirectorio de Trabajo: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Historial de Comandos Recientes:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Contenido del Directorio:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"french":     "Vous êtes un assistant de débogage shell sur macOS avec une conscience contextuelle améliorée. Ne sortez qu'un objet JSON avec le schéma : {\"explanation\":\"...\",\"command\":\"<shell>\"}. N'incluez pas de markdown ou de clés supplémentaires.\n\nCommande Échouée: {{.Command}}\nCode de Sortie: {{.ExitCode}}\nSortie Standard:\n{{.Stdout}}\nErreur Standard:\n{{.Stderr}}\n\nInformations de Contexte:\nRépertoire de Travail: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Historique des Commandes Récentes:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Contenu du Répertoire:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"german":     "Sie sind ein Shell-Debugging-Assistent auf macOS mit verbessertem Kontextbewusstsein. Geben Sie nur ein JSON-Objekt mit Schema aus: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Fügen Sie kein Markdown oder zusätzliche Schlüssel hinzu.\n\nFehlgeschlagener Befehl: {{.Command}}\nExit-Code: {{.ExitCode}}\nStandardausgabe:\n{{.Stdout}}\nStandardfehler:\n{{.Stderr}}\n\nKontextinformationen:\nArbeitsverzeichnis: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Aktuelle Befehlsverlauf:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Verzeichnisinhalt:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"italian":    "Sei un assistente di debug shell su macOS con consapevolezza del contesto migliorata. Emetti solo un oggetto JSON con schema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Non includere markdown o chiavi extra.\n\nComando Fallito: {{.Command}}\nCodice di Uscita: {{.ExitCode}}\nOutput Standard:\n{{.Stdout}}\nErrore Standard:\n{{.Stderr}}\n\nInformazioni di Contesto:\nDirectory di Lavoro: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Cronologia Comandi Recenti:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Contenuto Directory:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"portuguese": "Você é um assistente de depuração shell no macOS com consciência de contexto aprimorada. Emita apenas um objeto JSON com esquema: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Não inclua markdown ou chaves extras.\n\nComando Falhado: {{.Command}}\nCódigo de Saída: {{.ExitCode}}\nSaída Padrão:\n{{.Stdout}}\nErro Padrão:\n{{.Stderr}}\n\nInformações de Contexto:\nDiretório de Trabalho: {{.WorkingDirectory}}\nShell: {{.ShellType}}\n\n{{if .RecentCommands}}Histórico de Comandos Recentes:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Conteúdo do Diretório:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"russian":    "Вы помощник по отладке оболочки на macOS с улучшенным контекстным восприятием. Выводите только один JSON объект со схемой: {\"explanation\":\"...\",\"command\":\"<shell>\"}. Не включайте markdown или лишние ключи.\n\nНеудачная команда: {{.Command}}\nКод выхода: {{.ExitCode}}\nСтандартный вывод:\n{{.Stdout}}\nСтандартная ошибка:\n{{.Stderr}}\n\nИнформация о контексте:\nРабочий каталог: {{.WorkingDirectory}}\nОболочка: {{.ShellType}}\n\n{{if .RecentCommands}}История недавних команд:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}Содержимое каталога:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
			"arabic":     "أنت مساعد تصحيح أخطاء shell على macOS مع وعي سياقي محسن. أخرج فقط كائن JSON واحد بالمخطط: {\"explanation\":\"...\",\"command\":\"<shell>\"}. لا تتضمن markdown أو مفاتيح إضافية.\n\nالأمر الفاشل: {{.Command}}\nرمز الخروج: {{.ExitCode}}\nالإخراج القياسي:\n{{.Stdout}}\nخطأ قياسي:\n{{.Stderr}}\n\nمعلومات السياق:\nدليل العمل: {{.WorkingDirectory}}\nالغلاف: {{.ShellType}}\n\n{{if .RecentCommands}}تاريخ الأوامر الأخيرة:\n{{range $index, $cmd := .RecentCommands}}{{add $index 1}}. {{$cmd}}\n{{end}}{{end}}\n{{if .DirectoryListing}}محتوى الدليل:\n{{range .DirectoryListing}}{{.}}\n{{end}}{{end}}\nJSON:",
		},
	}
	return &Manager{prompts: defaultPrompts}
}

// GetPrompt returns a prompt by key.
func (m *Manager) GetPrompt(key string, lang string) (string, error) {
	if langPrompts, ok := m.prompts[key]; ok {
		if prompt, ok := langPrompts[lang]; ok {
			return prompt, nil
		}
		// Fallback to English if the specified language is not found
		if prompt, ok := langPrompts["en"]; ok {
			return prompt, nil
		}
	}
	return "", fmt.Errorf("prompt with key '%s' not found", key)
}
