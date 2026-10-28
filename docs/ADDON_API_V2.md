# MacroLeprechaun — Open Addon API

[English](#english) · [Українська](#українська)

## English

API 2 contract for the current implementation. [README](../README.md) · [Starter template](../modules/example/)

An addon is a directory of text files in `data/plagin/<id>/`. Edit its source directly: no EXE/DLL compilation, Go installation or application restart is required after editing the script.

### Start with the template

1. Copy `modules/example/` into `data/plagin/example/`.
2. Open Addons and choose Reload addons.
3. Enable the example and save changes.
4. The Example category provides a text field, a dropdown and a custom HTML dialog. `${example__clock}` and `${example__message}` receive values when polled.
5. Edit `plugin.ps1` and save. The next block execution reads the new code; changes to `manifest.json` require reloading addons.

Packaged examples reside in `build/data/plagin/pcstats/` and `build/data/plagin/notifications/` when using the full package. Their sources are in `addons/pcstats/` and `addons/notifications/`.

### Structure and manifest

Required files: `manifest.json` and a script. HTML pages are optional.

```json
{
  "id": "myaddon",
  "name": "My addon",
  "api": 2,
  "version": "1.0.0",
  "runtime": "powershell",
  "entry": "plugin.ps1",
  "description": "What this addon does",
  "settingsUI": "settings.html",
  "variables": [{"id": "state", "name": "State"}],
  "puzzleCategories": [{
    "id": "tools",
    "name": "My tools",
    "blocks": [
      {"id": "line", "name": "Text", "editor": "text"},
      {"id": "choice", "name": "Choice", "editor": "select", "options": ["one", "two"]},
      {"id": "dialog", "name": "Dialog", "editor": "button", "ui": "editor.html"}
    ]
  }]
}
```

Use a unique `id` containing ASCII letters, digits, `_` or `-`. Change it when copying a template. `entry` is a relative path inside the addon, such as `src/plugin.py`; escaping the directory or pointing outside it through a symlink is forbidden. Optional `args` are passed as separate arguments.

An addon may declare **at most one puzzle category**, with multiple blocks. A variables-only addon needs no category. Block type: `plugin:<addon>/<category>/<block>`.

| Runtime | Script | PC requirement |
| --- | --- | --- |
| `powershell` | `plugin.ps1` | Built-in Windows PowerShell; examples need no separate installation |
| `python` | `plugin.py` | Python 3 in PATH; Windows `py -3` is also supported |
| `javascript` | `plugin.js`, `.cjs`, `.mjs` | Node.js in PATH |

PowerShell files are read as UTF-8 with or without BOM. Execution is hidden, without a profile, and uses STA on Windows. Execution policy is set only for the process; system/organization restrictions still apply. Python stdin/stdout use UTF-8.

The working directory is the addon root even when entry is inside `src/`. Its absolute path is provided as `MACROLEPRECHAUN_ADDON_DIR`. PowerShell scripts should use `$env:MACROLEPRECHAUN_ADDON_DIR`; code is loaded through ScriptBlock, so do not rely on `$PSScriptRoot` or `$PSCommandPath`.

### Requests and responses

Each process receives one JSON object through stdin until EOF, writes one JSON response to stdout and exits. stdout must contain only JSON; write diagnostics to stderr, without greetings, tables or debug lines in stdout.

```json
{
  "api": 2,
  "op": "execute",
  "block": "dialog",
  "value": {"title": "Hello"},
  "settings": {"enabled": true},
  "variables": {"volume": "50", "system__time": "12:00:00"},
  "requestedVariables": null,
  "host": {"keyboardLayout": "uk-UA"}
}
```

Success: `{"ok":true,"variables":{"state":"ready"}}`. Failure: `{"ok":false,"error":"Reason"}`. Success requires exit code 0. A script error or nonzero exit code stops the current macro branch and is logged. Response size is limited to 8192 bytes.

- `op=execute` runs a block. Text/select editors receive a string, including `"123"`; button editors receive the JSON value saved by their HTML form.
- `op=variables` polls needed variables with an approximately two-second cache and a five-second timeout per call. Do not show windows or perform macro actions here. Addons without declared variables are not polled.
- `settings` contains the saved settingsUI object, or null before settings are saved.
- `variables` is a snapshot of available app variables.
- `host.keyboardLayout` is the active window's keyboard layout, rather than the script process layout, or `unavailable`.

Only short variable IDs declared in the manifest are accepted in responses. `state` becomes `${myaddon__state}`. Responses cannot modify system variables or another addon's namespace. Values are converted to strings. Return the requested short IDs; missing keys preserve their last values rather than deleting them. Availability depends on addon enablement and `enableSetting`; before the first value the host displays `—`.

Each request starts a fresh process and reads the current script. Interpreter globals do not persist. Use snapshot values or your own JSON file for state. On Windows, stopping a macro cancels its active script process tree; already-delivered asynchronous notifications remain until their own timer expires.

Scripts run with the current user's permissions and are not isolated from the OS. The iframe sandbox applies only to the HTML interface.

### Minimal scripts

PowerShell, `plugin.ps1`:

```powershell
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$value = 'ready'
if ($request.op -eq 'execute') { $value = [string]$request.value }
@{ ok = $true; variables = @{ state = $value } } |
    ConvertTo-Json -Compress -Depth 10
```

Python, `plugin.py` — change runtime/entry accordingly:

```python
import json
import sys
request = json.load(sys.stdin)
print(json.dumps({"ok": True, "variables": {"state": "ready"}}, ensure_ascii=False))
```

JavaScript for Node.js, `plugin.js`:

```js
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const request = JSON.parse(input);
  console.log(JSON.stringify({ok: true, variables: {state: 'ready'}}));
});
```

### Blocks, HTML and the addon menu

`editor=text` provides a text field, `select` provides an `options` dropdown and `button` opens custom HTML. Forms can use fields, tabs, canvas, CSS and JavaScript. See `modules/example/editor.html` for a working JSON-saving form.

`settingsUI` adds a settings button to the addon card and a menu entry for an enabled addon. It can contain settings, instructions and a description. Its saved object is stored in `Config.moduleSettings[addon-id]` after the main Save changes action.

The iframe uses `allow-scripts allow-forms` without same-origin. Inline JS/CSS and data: images are allowed. Network fetches, external scripts/styles, navigating form submissions and access to the main app DOM/cookies are blocked. Embed code, styles and images directly in HTML.

```js
addEventListener('message', e => {
  if (e.source !== parent || !e.data) return;
  if (e.data.type === 'init') {
    // value: addon settings or block value
    // variables: snapshot; language: locale code; accent: CSS color
  }
});
parent.postMessage({type: 'save', value: {title: 'Hello'}}, '*');
parent.postMessage({type: 'cancel'}, '*');
parent.postMessage({type: 'variables'}, '*');
// Reply: {type:'variables',value:{...}}
parent.postMessage({type: 'pick', kind: 'file', requestId: 'file1'}, '*');
// Reply: {type:'picked',requestId:'file1',path:'C:\\...'} or error
```

The host accepts messages only from the currently open iframe. Save updates the draft and closes the dialog. Block edits still need Apply and the main Save changes action. settingsUI values must be objects; block values may be any JSON value.

### Included addons

**PC statistics:** `plugin.ps1` queries CPU/RAM/GPU. CPU uses the Windows `_Total` performance class, RAM reports used physical memory percentage and GPU is an aggregate engine utilization estimate. Missing Windows/driver data yields `unavailable`. The host provides the active keyboard layout. All metrics are disabled by default and selected in settings.html. Variables: `${pcstats__cpu}`, `${pcstats__ram}`, `${pcstats__gpu}`, `${pcstats__keyboard}`.

**Notifications:** `plugin.ps1` launches a separate hidden PowerShell process; `window.ps1` implements the window. `custom.html` and `windows.html` configure blocks; `about.html` describes the addon. Custom notifications support nine positions, colors and a 0.1–3600 second timer, without taking focus. A small C# snippet is compiled in memory by PowerShell for non-activating window behavior; no separately compiled DLL/EXE is shipped. System notifications use NotifyIcon and depend on Windows notification settings. Asynchronous GUI failures go to `notification-error.log` next to the script.

### Legacy executable addons

Use the script folders `data/plagin/pcstats` and `data/plagin/notifications` instead of their old addon executables, reload addons and save. Preserve unrelated data, macros and settings. Legacy third-party API 1/2 addons with `executable` remain supported; new script addons use `runtime` + `entry` and omit `executable`.

### Controls and multiple inputs

Native widgets support buttons, toggles and sliders. Addon blocks run only in the connected branch of the activated input. A slider input variable is optional; if selected, its value is present in the snapshot.

A macro can contain up to eight input nodes, one per split-button zone. `/api/control` uses `panel`, `control`, `value`; `control` identifies a specific zone and the host selects its input. This app endpoint is not exposed through the addon HTML bridge. The addon stdin/stdout protocol remains the same.

### On-demand variable polling

For `op: "variables"`, `requestedVariables: ["cpu", "ram"]` lists short IDs. Query and return only needed metrics. Null or an absent field means an unfiltered legacy call. If no available variables are needed, the host does not start the provider process. Older addons remain compatible but may query unnecessary metrics internally.

A variable can declare `"enableSetting": "cpu"`; it is available only when `moduleSettings[addonID].cpu === true`. Without this field, availability follows addon enablement. Last values are retained; before the first poll the value is `—`.

References such as `${addon__variable}` in labels/block values and variable names in JSON settings determine demand automatically. For dependencies hidden in code, declare `"usesVariables": ["pcstats__cpu", "system__network"]` at the manifest root. The host requests these values for macros using the addon's blocks. This grants no permission to write another namespace.

The internal admin API `GET /api/config` returns a macro catalog with `lazy: true`, `file` and `summary`, without block bodies. Admins obtain a body through `GET /api/macro?id=...`; save accepts an unchanged catalog entry or a complete edited body. Do not change service fields `file`/`summary`; the host validates them. These endpoints are not additional iframe bridge messages.

### Supported integration map

| Integration | Declaration / call | Available behavior |
| --- | --- | --- |
| Script execution | `runtime`, `entry`, optional `args` | Fresh process per request, JSON over stdin/stdout |
| Puzzle category | `puzzleCategories` | Zero or one category per addon; multiple blocks |
| Text field | `editor: "text"` | String value at execute |
| Dropdown | `editor: "select"`, `options` | Selected string |
| Block dialog | `editor: "button"`, `ui` | Custom HTML form saving JSON |
| Settings / description | `settingsUI` | Addon card button and menu entry; settings object in requests |
| Own variables | `variables: [{id,name,enableSetting?}]` | `${addon__id}` namespace; writes through script responses |
| External variable dependencies | `usesVariables` | Declares demand, not write permissions |
| Current app data | `variables`, `host.keyboardLayout` | Value snapshot and active keyboard layout |
| File picker from HTML | `pick` message | file/folder/program/output dialog returning a path |
| Files and external services | Script and runtime libraries | Current-user OS access; author implements protocols/authentication |

### Complete manifest fields

| Field | Meaning |
| --- | --- |
| `id` | Unique addon ID used in block types and variable names |
| `name`, `version`, `description` | Display name, addon version and description |
| `api` | `2` for new script addons |
| `runtime` | `powershell`, `python` or `javascript` |
| `entry` | Relative path to an existing file inside the addon |
| `args` | Script arguments, passed separately |
| `settingsUI` | Relative path to a declared HTML page |
| `variables` | Array of `{id,name,enableSetting?}`; enableSetting names a boolean settings key |
| `usesVariables` | Full dependency names without `${}` |
| `puzzleCategories` | Array containing at most one `{id,name,blocks}` |
| `blocks` | Category array of `{id,name,editor,options?,ui?}` |
| `actions` | Legacy module action list; does not create puzzle blocks by itself |
| `executable` | Legacy binary compatibility; omit with runtime/entry |

The host sets `error` and `runtimeError` in status data; these are not manifest settings. Keep IDs unique within their respective scopes. Scanning matches `data/plagin/*/manifest.json`; nested manifests are not discovered automatically.

### Complete request fields

| Field | Type / content |
| --- | --- |
| `api` | Number `2` |
| `op` | `execute` or `variables` |
| `block` | Short block ID for execute; empty for variables |
| `value` | String for text/select; JSON for button; null for variables |
| `settings` | Saved settings object or null |
| `variables` | Full names mapped to string values at request time |
| `requestedVariables` | Short IDs to poll; may be null in other calls |
| `host.keyboardLayout` | Active layout string or `unavailable` |

`variables` is optional in a successful response. Other returned fields are not host commands: returning `panels`, `macros`, `settings` or `notifications` does not automatically modify the application.

### Complete HTML bridge

| Direction | Message | Result |
| --- | --- | --- |
| host → iframe | `{type:"init",value,variables,language,accent}` | Initial dialog state |
| iframe → host | `{type:"save",value}` | Save draft and close |
| iframe → host | `{type:"cancel"}` | Close without saving the form |
| iframe → host | `{type:"variables"}` | Request editor snapshot |
| host → iframe | `{type:"variables",value:{...}}` | Snapshot response |
| iframe → host | `{type:"pick",kind,requestId}` | Open system picker |
| host → iframe | `{type:"picked",requestId,path}` | Selected path, empty on cancellation |
| host → iframe | `{type:"picked",requestId,error}` | Picker failure |

`kind` accepts only `file`, `folder`, `program`, `output`. Choose a `requestId` to correlate replies. Check `e.source === parent` and message structure in the form. The isolated iframe origin requires `parent.postMessage(message, '*')`.

HTML has no messages for running/stopping macros, writing runtime variables, listing terminals/audio devices or editing panels. Save is not execution. Network access from the iframe is blocked by CSP; use the script for network integrations. Pages must be declared in settingsUI or block.ui. External JS/CSS and relative resources are not loaded; use inline code/styles and data: images.

### Lifecycle and limits

1. Reload scans manifests and validates entry. Enablement and settings persist through the main Save action.
2. Executing a connected block starts a fresh script process. Disabled or unavailable addons cause a block error.
3. Execute inherits the active macro context; API 2 has no separate fixed per-block timeout. The script must finish or be cancelled by stopping the macro.
4. Providers poll on demand with an approximately two-second cache and up to five seconds per call, within the overall refresh context. This is not a real-time scheduler.
5. Unused variables pause while keeping their last values. Opening the variables view temporarily requests available metrics. Labels, block parameters and usesVariables also create demand.
6. stdout and stderr each have an 8192-byte buffer. Oversized JSON is truncated and fails parsing. Store large content in files and return a short status/path through a declared variable.
7. No automatic pip/npm dependency installation, persistent background process, event subscription or manifest hot reload is provided. The author/user supplies runtimes and dependencies. Code changes apply on the next call; manifest changes require reload.
8. Calls from different macros can overlap. Protect shared state files with locking/atomic writes.

### What the API does not provide

This contract does not add custom phone button types, arbitrary main-editor tabs, new panel modes, host socket routes or native libraries through the manifest. Radial menu styles use a [separate format](FLOATING_THEMES.md). Scripts may connect to services, devices and libraries supported by their runtime and OS permissions, but the host provides no universal adapters. The variables snapshot is not a streaming store and contains neither complete macro bodies nor a panel list.

### Testing your addon

- Start with a text block and a valid JSON response without extra stdout output.
- Test variables separately from execute, an unknown block and missing settings.
- Test dialog save/cancel, reopening and cancelled file selection.
- Test disabling, missing runtime, cancelling a long macro and restarting it.
- Write diagnostics to stderr; inspect app errors and `data/application.log`.

Contract sources: [execution](../desktop/plugins_v2.go), [runtime launcher](../desktop/script_runtime.go), [manifest scanning](../desktop/engine.go), [polling](../desktop/variable_demand.go), [HTML bridge](../desktop/web/plugin-editor.js).

---

## Українська

Контракт API 2 для поточної реалізації. [README](../README.md) · [Шаблон](../modules/example/)

Доповнення — звичайна папка з текстовими файлами. Код можна змінити безпосередньо в `data/plagin/<id>/`. Збирати EXE або DLL, встановлювати Go чи перезапускати програму після редагування коду не потрібно.

### Почати з готового шаблону

1. Скопіюйте `modules/example/` у `data/plagin/example/`.
2. У розділі «Доповнення» натисніть «Перечитати доповнення».
3. Увімкніть приклад і збережіть зміни.
4. У редакторі макроса з’явиться категорія «Приклад»: рядок, список і кнопка власного HTML-вікна. Змінні `${example__clock}` і `${example__message}` з’являться після опитування.
5. Відкрийте `plugin.ps1` у редакторі, змініть поведінку та збережіть. Наступний запуск пазла використає новий код. Для зміни `manifest.json` потрібне лише повторне читання доповнень.

Готові встановлені приклади: `build/data/plagin/pcstats/` і `build/data/plagin/notifications/`. Їхні оригінали — `addons/pcstats/` та `addons/notifications/`.

### Структура й маніфест

Мінімум — `manifest.json` і файл коду. HTML додається за потреби.

```json
{
  "id": "myaddon",
  "name": "Моє доповнення",
  "api": 2,
  "version": "1.0.0",
  "runtime": "powershell",
  "entry": "plugin.ps1",
  "description": "Опис можливостей",
  "settingsUI": "settings.html",
  "variables": [{"id": "state", "name": "Стан"}],
  "puzzleCategories": [{
    "id": "tools",
    "name": "Мої інструменти",
    "blocks": [
      {"id": "line", "name": "Рядок", "editor": "text"},
      {"id": "choice", "name": "Список", "editor": "select", "options": ["one", "two"]},
      {"id": "dialog", "name": "Вікно", "editor": "button", "ui": "editor.html"}
    ]
  }]
}
```

`id` має бути унікальним: ASCII-літери, цифри, `_`, `-`. Копіюючи шаблон для нового доповнення, змініть його ID. `entry` — відносний шлях до файла всередині папки доповнення; дозволено, наприклад, `src/plugin.py`. Переходи за межі папки й symlink на сторонній файл для entry заборонені. `args` — необов’язковий масив додаткових аргументів, кожен передається окремим значенням.

Одне доповнення може оголосити **не більше однієї категорії** пазлів. Для доповнення лише зі змінними категорія не потрібна. Кількість пазлів у категорії довільна в межах можливостей редактора. Type пазла: `plugin:<addon>/<category>/<block>`.

| runtime | Код | Що потрібно на ПК |
| --- | --- | --- |
| `powershell` | `plugin.ps1` | Системний Windows PowerShell; окрема установка для готових прикладів не потрібна |
| `python` | `plugin.py` | Встановлений Python 3 у PATH; підтримується також Windows `py -3` |
| `javascript` | `plugin.js`, `.cjs`, `.mjs` | Встановлений Node.js у PATH |

PowerShell-файли завантажуються як UTF-8, з BOM або без нього. Запуск прихований, без профілю, у STA на Windows. Політика виконання задається лише для процесу, не змінює системні налаштування. Обмеження організації/системи продовжують діяти. Python працює з UTF-8 stdin/stdout.

Робоча директорія процесу — корінь доповнення, навіть якщо `entry` лежить у `src/`. Абсолютний шлях до цього кореня передається у змінній середовища `MACROLEPRECHAUN_ADDON_DIR`. У PowerShell використовуйте `$env:MACROLEPRECHAUN_ADDON_DIR` для власних файлів; код читається через ScriptBlock, тому не покладайтеся на автоматичні `$PSScriptRoot`/`$PSCommandPath`.

### Запит і відповідь

Один запуск процесу — один JSON-об’єкт у stdin до EOF, одна JSON-відповідь stdout і завершення. stdout повинен містити лише JSON, діагностика — stderr. Ніякого виводу привітань, форматованих таблиць чи debug-рядків у stdout.

```json
{
  "api": 2,
  "op": "execute",
  "block": "dialog",
  "value": {"title": "Привіт"},
  "settings": {"enabled": true},
  "variables": {"volume": "50", "system__time": "12:00:00"},
  "host": {"keyboardLayout": "uk-UA"}
}
```

```json
{"ok":true,"variables":{"state":"ready"}}
```

Або `{"ok":false,"error":"Причина"}`. Код завершення для успішної відповіді — 0. Помилка скрипта чи ненульовий exit code зупиняє поточну гілку макроса й записується в журнал. Відповідь обмежена 8192 байтами.

- `op=execute` — запуск конкретного пазла. `editor=text/select` отримує `value` як рядок; навіть `"123"` лишається рядком. `editor=button` отримує JSON-значення з HTML-форми.
- `op=variables` — опитування постачальника змінних приблизно раз на 2 секунди. Не показуйте тут вікон і не виконуйте дії макроса. Таймаут — 5 секунд. Для доповнення без оголошених змінних опитування не виконується.
- `settings` — збережений об’єкт із settingsUI, або null, якщо налаштування ще не зберігали.
- `variables` — знімок доступних змінних програми.
- `host.keyboardLayout` — мова розкладки активного вікна (не прихованого процесу скрипта), або `unavailable`.

Повернення змінних приймається лише для коротких ID, оголошених у manifest. `state` стає `${myaddon__state}`. Системні змінні та чужі простори відповіддю змінити не можна. Значення перетворюється на рядок. Повертайте значення запитаних коротких ID. Відсутній у відповіді ключ не стирає попереднє значення: host зберігає останнє отримане. Доступність визначається ввімкненням доповнення та `enableSetting`; до першого отримання показується `—`.

Кожен запит створює новий процес і читає поточний файл коду; глобальні змінні інтерпретатора між запусками не зберігаються. Для стану використовуйте попередні значення зі snapshot або власний JSON-файл у папці доповнення. У Windows зупинка активного макроса скасовує дерево активного процесу скрипта. Уже доставлені асинхронні сповіщення живуть до свого таймера.

Скрипт виконується з правами поточного користувача. Сам протокол не ізолює код від ОС; ізоляція iframe стосується лише HTML-інтерфейсу.

### Мінімальні скрипти

PowerShell, `plugin.ps1`:

```powershell
$request = [Console]::In.ReadToEnd() | ConvertFrom-Json
$value = 'ready'
if ($request.op -eq 'execute') { $value = [string]$request.value }
@{ ok = $true; variables = @{ state = $value } } |
    ConvertTo-Json -Compress -Depth 10
```

Python, `plugin.py` (змініть runtime/entry у маніфесті):

```python
import json
import sys
request = json.load(sys.stdin)
print(json.dumps({"ok": True, "variables": {"state": "ready"}}, ensure_ascii=False))
```

JavaScript для Node.js, `plugin.js`:

```js
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  const request = JSON.parse(input);
  console.log(JSON.stringify({ok: true, variables: {state: 'ready'}}));
});
```

### Пазли, HTML та меню доповнення

`editor=text` дає рядок, `select` — список `options`, `button` — кнопку, яка відкриває власний HTML. У формі можна використовувати довільні поля, вкладки, canvas, CSS та JavaScript. Форму з готовим збереженням JSON дивіться у `modules/example/editor.html`.

`settingsUI` додає кнопку до картки доповнення та пункт під меню для ввімкненого доповнення. Тут можна розмістити налаштування, інструкцію й опис. Збережений об’єкт потрапляє до `Config.moduleSettings[addon-id]` після загального «Зберегти зміни».

Iframe має `allow-scripts allow-forms`, без same-origin. Inline JS/CSS та data:-зображення дозволені. Мережеві fetch, зовнішні JS/CSS, форми з переходом і доступ до DOM/cookies основної програми не дозволені. CSS і JavaScript включайте безпосередньо у HTML.

```js
addEventListener('message', e => {
  if (e.data.type === 'init') {
    // e.data.value: налаштування доповнення або значення пазла
    // e.data.variables: snapshot змінних
    // e.data.language: код мови
    // e.data.accent: акцентний CSS-колір
  }
});
parent.postMessage({type: 'save', value: {title: 'Привіт'}}, '*');
parent.postMessage({type: 'cancel'}, '*');
parent.postMessage({type: 'variables'}, '*');
// відповідь: {type:'variables',value:{...}}
parent.postMessage({type: 'pick', kind: 'file', requestId: 'file1'}, '*');
// kind: file, folder, program, output
// відповідь: {type:'picked',requestId:'file1',path:'C:\\...'} або error
```

Програма приймає повідомлення лише від iframe відкритого вікна. `save` змінює чернетку і закриває вікно. Для пазла потрібні також «Застосувати» та загальне «Зберегти зміни». Для settingsUI `value` має бути об’єктом; для пазла допускається довільне JSON-значення.

### Готові доповнення

**Статистика ПК:** `plugin.ps1` містить логіку запитів CPU/RAM/GPU. CPU — Windows performance class `_Total`, RAM — частка використаної фізичної пам’яті, GPU — агрегована оцінка зайнятості engine. Якщо Windows/драйвер не надає даних, повертається `unavailable`. Мову активної клавіатури передає host. Усі показники вимкнені за замовчуванням; вибір — у settings.html. Імена змінних залишилися `pcstats__cpu/ram/gpu/keyboard`.

**Сповіщення:** `plugin.ps1` запускає окремий прихований PowerShell-процес, а `window.ps1` містить весь код вікна. `custom.html`/`windows.html` — форми пазлів, `about.html` — опис. Кастомне вікно має 9 положень, кольори й таймер 0,1–3600 секунд; не забирає фокус. Короткий відкритий C#-фрагмент у window.ps1 завантажується PowerShell у пам’ять для режиму без активації — збирати чи постачати окрему DLL/EXE не потрібно. Системне сповіщення лишається NotifyIcon, його показ залежить від налаштувань Windows. Помилка асинхронного GUI записується у `notification-error.log` поруч зі скриптом.

### Перехід з EXE-доповнень

1. Замініть EXE основної програми на 0.13.0 або новішу.
2. Замініть лише папки `data/plagin/pcstats` і `data/plagin/notifications` відповідними папками з архіву; старі `pcstats.exe`/`notifications.exe` у цих папках більше не потрібні.
3. Натисніть «Перечитати доповнення» та збережіть зміни. ID, типи пазлів і налаштування збережено сумісними.

Решту `data`, власні макроси й налаштування не замінюйте. Старі сторонні API 1/2 доповнення з полем `executable` залишаються сумісними; новий формат використовує `runtime` + `entry`, без `executable`.


### Контроли 0.13

Нативні віджети тепер підтримують кнопку, перемикач і повзунок; пазл доповнення виконується у відповідній приєднаній гілці input. Змінна input-повзунка необов’язкова, а її значення присутнє у snapshot, якщо вибрана. Типи пазлів та протокол API 2 не змінювалися.


### 0.14: кілька входів у розділеній кнопці

Макрос може містити до 8 input-вузлів, по одному на зону розділеної кнопки. В API `/api/control` як і раніше передають `panel`, `control`, `value`; `control` — ID конкретної зони. Програма сама вибирає її input. Пазл доповнення виконується лише у приєднаній до цього input гілці. Протокол stdin/stdout доповнень не змінювався.


### 0.15: опитування змінних на вимогу

У `op: "variables"` host передає `requestedVariables: ["cpu", "ram"]` — короткі ID змінних цього доповнення. Повертайте тільки потрібні показники й виконуйте лише відповідні запити. `null` або відсутнє поле означає старий виклик без фільтра. Коли не потрібна жодна доступна змінна, host взагалі не запускає процес провайдера. Старі доповнення сумісні, але можуть опитувати зайві показники всередині одного виклику.

Для змінної маніфесту дозволено `"enableSetting": "cpu"`: змінна доступна лише якщо `moduleSettings[addonID].cpu === true`. Якщо поле відсутнє, доступність визначається ввімкненням доповнення. Host зберігає останнє отримане значення, а до першого опитування показує `—`.

Посилання `${addon__variable}` в підписах/значеннях пазлів та назви змінних у JSON налаштувань визначають потребу автоматично. Якщо залежності заховані у коді доповнення, оголосіть на верхньому рівні маніфесту `"usesVariables": ["pcstats__cpu", "system__network"]`. Host активує їх для макросів із пазлами цього доповнення. Це декларація залежностей, а не дозвіл записувати чужі змінні.

`GET /api/config` тепер повертає каталог макросів із `lazy: true`, `file` і `summary`, без тіл пазлів. Адміністратор отримує тіло через `GET /api/macro?id=...`; при збереженні можна передавати незмінений каталожний запис або повне відредаговане тіло. Не змінюйте службові `file`/`summary`: host перевіряє їх за своїм каталогом. Протокол виконання доповнень та `/api/control` не змінено.


### Карта всіх підтримуваних підключень

| Підключення | Як оголосити / викликати | Що доступно |
| --- | --- | --- |
| Код виконання | `runtime` + `entry` + необов’язкові `args` | Окремий процес на кожен запит, JSON через stdin/stdout |
| Категорія пазлів | `puzzleCategories` | Нуль або одна категорія на доповнення; кілька блоків у категорії |
| Просте текстове поле | блок `editor: "text"` | Рядок `value` при execute |
| Випадаючий список | `editor: "select"`, `options: ["a", "b"]` | Вибране рядкове значення |
| Вікно пазла | `editor: "button"`, `ui: "editor.html"` | Власна HTML-форма, збереження JSON |
| Загальні налаштування / опис | `settingsUI: "settings.html"` | Кнопка картки доповнення та пункт меню; об’єкт `settings` у скрипті |
| Власні змінні | `variables: [{id,name,enableSetting?}]` | Простір `${addon__id}`, читання в UI/макросах; запис лише через відповідь скрипта |
| Залежності від інших змінних | `usesVariables: ["other__state"]` | Повідомляє host, що показники потрібні; не надає прав запису |
| Поточні дані програми | `variables`, `host.keyboardLayout` у запиті | Знімок значень і розкладка активного вікна |
| Вибір файлів з HTML | повідомлення `pick` | Системний діалог file/folder/program/output; повертається шлях |
| Файли та зовнішні сервіси | Код скрипта й бібліотеки його runtime | Доступ ОС поточного користувача; власні протоколи/авторизацію реалізує автор |

#### Повний список полів маніфесту

| Поле | Значення |
| --- | --- |
| `id` | Унікальний ID; використовується в типах пазлів і просторі змінних |
| `name`, `version`, `description` | Назва, версія та опис картки |
| `api` | Для нового відкритого доповнення — `2` |
| `runtime` | Тільки `powershell`, `python`, `javascript` |
| `entry` | Відносний шлях до наявного файла всередині доповнення |
| `args` | Масив аргументів скрипта, кожен передається окремо |
| `settingsUI` | Відносний шлях до оголошеної HTML-сторінки |
| `variables` | Масив `{id, name, enableSetting?}`; enableSetting — ключ boolean у settings |
| `usesVariables` | Масив повних імен залежностей без `${}` |
| `puzzleCategories` | Масив до однієї `{id,name,blocks}` |
| `blocks` | Масив `{id,name,editor,options?,ui?}` у категорії |
| `actions` | Список дій сумісності зі старим викликом модулів; сам собою пазлів не створює |
| `executable` | Старий двійковий формат сумісності; не вказувати разом з runtime/entry |

`error` і `runtimeError` у даних статусу виставляє host: це не налаштування маніфесту. Уникайте однакових ID доповнень, категорій, блоків та змінних у відповідних просторах. Каталог сканується як `data/plagin/*/manifest.json` — вкладені маніфести автоматично не шукаються.

#### Усі поля запиту виконання

| Поле | Тип і значення |
| --- | --- |
| `api` | Число `2` |
| `op` | `execute` або `variables` |
| `block` | Короткий ID блока при execute; порожній рядок при variables |
| `value` | Рядок для text/select; JSON для button; null при variables |
| `settings` | Об’єкт збережених налаштувань або null |
| `variables` | Об’єкт повних імен і рядкових значень; знімок на момент запиту |
| `requestedVariables` | Масив коротких ID для опитування; може бути null при інших викликах |
| `host.keyboardLayout` | Рядок мови активної розкладки або `unavailable` |

Успішна відповідь: `{"ok":true,"variables":{"state":"ready"}}`. `variables` можна не повертати. Відповідь `{"ok":false,"error":"Опис"}` означає помилку. Довільні інші поля host не трактує як команди: повернення `panels`, `macros`, `settings` чи `notifications` не змінить програму автоматично.

#### Повний HTML-міст

| Напрямок | Повідомлення | Результат |
| --- | --- | --- |
| host → iframe | `{type:"init",value,variables,language,accent}` | Початковий стан після завантаження iframe |
| iframe → host | `{type:"save",value}` | Передає чернетку й закриває вікно |
| iframe → host | `{type:"cancel"}` | Закриває вікно без збереження цієї форми |
| iframe → host | `{type:"variables"}` | Запит поточного знімка редактора |
| host → iframe | `{type:"variables",value:{...}}` | Відповідь на запит змінних |
| iframe → host | `{type:"pick",kind,requestId}` | Запит системного діалогу |
| host → iframe | `{type:"picked",requestId,path}` | Шлях, порожній при скасуванні |
| host → iframe | `{type:"picked",requestId,error}` | Помилка діалогу |

`kind` приймає лише `file`, `folder`, `program`, `output`. `requestId` придумує автор форми для зіставлення відповідей. Host перевіряє джерело повідомлення; у власному HTML теж перевіряйте `e.source === parent` і структуру `e.data` перед обробкою. Через ізольоване походження iframe використовується `parent.postMessage(message, '*')`.

HTML не має довільного доступу до API програми: немає повідомлень для запуску/зупинки макроса, запису runtime-змінної, переліку терміналів/аудіопристроїв або редагування панелей. `save` не є командою виконання. Мережа з iframe заблокована CSP; підключення до мережі реалізовується скриптом, а не fetch у формі. Сторінка має бути оголошена в `settingsUI` або `block.ui`; довільні шляхи не обслуговуються. Зовнішні JS/CSS і відносні ресурси не підвантажуються — вбудовуйте код/стилі й data:-зображення в HTML.

#### Життєвий цикл та межі

1. Перечитування знаходить маніфест і перевіряє entry. Увімкнення та налаштування персистуються загальним збереженням.
2. Виконання приєднаного пазла запускає новий процес скрипта. Якщо доповнення вимкнене або недоступне, пазл завершується помилкою.
3. Виклик `execute` використовує контекст активного макроса; окремого сталого таймауту API 2 для кожного пазла немає. Скрипт має завершитися сам або бути скасований зупинкою макроса.
4. Постачальники змінних опитуються на вимогу з кешем близько 2 секунд і таймаутом до 5 секунд на виклик, у межах загального контексту оновлення. Це не точний таймер реального часу.
5. Незатребувані змінні призупиняються зі збереженням останнього значення; перегляд розділу змінних тимчасово створює потребу в доступних показниках. Потребу також створюють підписи, параметри пазлів і usesVariables.
6. stdout і stderr обмежені буферами по 8192 байти; великий JSON буде обрізано і не розібрано. Довгі дані зберігайте у файлах, повертайте короткий стан/шлях у власній змінній.
7. Немає автоматичного встановлення pip/npm-модулів, постійного фонового процесу, підписки на події чи hot reload маніфесту. Залежності й інтерпретатор готує користувач/автор; змінений код читається наступним викликом, маніфест — після перечитування.
8. Не припускайте послідовного виконання всіх викликів: різні макроси можуть працювати одночасно. Власний спільний файл стану потребує блокування/атомарного запису.

#### Що API не надає

Цей контракт не додає власні типи кнопок телефона, довільні вкладки основного редактора, нові режими панелей, власні сокетні маршрути host або нативні бібліотеки через маніфест. Стилі кругового меню мають [окремий формат](FLOATING_THEMES.md). Сторонні сервіси, пристрої й бібліотеки можна підключати з коду скрипта, якщо їх підтримує обраний runtime та дозволяє ОС; готових універсальних адаптерів у host немає. Знімок variables не є потоковим сховищем і не містить повних тіл макросів чи списку панелей.

#### Перевірка доповнення

- Спочатку перевірте text-пазл і успішний JSON без зайвих рядків у stdout.
- Перевірте `op=variables` окремо від execute, невідомий block та відсутні налаштування.
- Перевірте UI save/cancel, повторне відкриття форми й скасування file picker.
- Перевірте вимкнення, пропущений інтерпретатор, зупинку довгого макроса та повторний запуск.
- Діагностику пишіть у stderr; помилки host дивіться в інтерфейсі та `data/application.log`.

Джерело контракту: [виконання](../desktop/plugins_v2.go), [запуск runtime](../desktop/script_runtime.go), [сканування маніфестів](../desktop/engine.go), [опитування](../desktop/variable_demand.go), [HTML-міст](../desktop/web/plugin-editor.js).
