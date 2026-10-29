# MacroLeprechaun

[English](#english) · [Українська](#українська)

## English

Your personal Windows automation: build macros from puzzle blocks and launch them from an Android phone, an edge widget or a floating radial menu.

[Project website](https://macroleprechaun-web.sanya-1998-16-4.workers.dev/) · [Addon API](docs/ADDON_API_V2.md) · [Support development](https://send.monobank.ua/jar/3rqmBsdneV)

### Features

- **Three control modes.** A smart panel on your phone, an edge widget with an action list and a radial menu at your cursor. Each mode can be disabled separately; disabling the smart panel stops the phone control server.
- **Puzzle editor.** Magnetic connections, a main branch starting at input, separate true/false branches, group selection and movement. Disconnected blocks do not run.
- **Text tools.** Insert text without the clipboard, write to the clipboard, replace selected text, select text and find the match nearest the cursor.
- **Windows actions.** Commands through available terminals, files and programs with arguments, folders, audio and microphone controls, browser commands, hotkeys and mouse clicks.
- **Macro actions.** Delays, conditions, waits, loops, parallel macro execution, stopping macros and reading or writing variables. Loops require an explicit setting; a stop-all button is available.
- **Panels.** A hierarchy with a single parent per panel, panel switching, back navigation and return to the root in the relevant control mode.
- **Buttons, toggles and sliders.** Colors, icons from files, backgrounds, image scaling, animations and effects. Sliders support orientation and activation on release or during movement at a configurable interval.
- **Smart panel grid.** Free placement, resizing into empty cells, splitting a button into multiple zones and merging selected buttons while choosing which macro to keep.
- **Live labels.** Use `${volume} ${system__time}`. Custom variables, system time, date, network name and addon variables are available.
- **Open-source addons.** PowerShell, Python or JavaScript without compilation; a custom puzzle category, variables and HTML settings dialogs. Included examples cover PC statistics and notifications.
- **Radial menu themes.** Rendering and themes based on Kando, with editable files in `data/effects/floating/`.
- **Local operation.** Your phone connects to the PC over the local network; control does not require a cloud account. Macros load from disk on demand, persistent RAM caching is optional and unused external variables can pause polling.

### Download and run

1. Download `MacroLeprechaun.exe`; download `MacroLeprechaun.apk` for your phone.
2. Place the EXE in a writable folder and launch it. The app creates and uses `data` beside the EXE. If using a complete package, preserve its folder structure.
3. The editor and radial menu require **Microsoft Edge WebView2 Runtime**. The app uses its own profile without your personal browser extensions.
4. To connect a phone, enable the smart panel, open “Settings → Connect phone” and generate a pairing code. Both devices must share a reachable local network; allow the app through Windows Firewall for that network.
5. Create a button, open “Edit macro”, connect blocks to input, apply and save your changes.

**Requirements:** Windows 10/11 x64; Android 8.0 or later for the APK. The phone acts as a remote: actions run on the PC, so the desktop app must remain running.

### Data

| Directory | Purpose |
| --- | --- |
| `data/macros/` | Macro JSON files |
| `data/setting/` | SQLite app and button settings, interface runtime data |
| `data/icon/` | Icon files |
| `data/image/` | Background images |
| `data/plagin/` | Open-source addons |
| `data/effects/floating/` | Floating panel themes |
| `data/locales/` | Localization JSON files |

To back up your setup, close the app and copy `data`. Do not include your personal working `data` with macros, pairing information and settings in a public release. Startup diagnostics are stored in `data/application.log`.

### Addons and themes

- **[Full addon API](docs/ADDON_API_V2.md)** — manifest, runtimes, stdin/stdout, variables, HTML bridges and access limits.
- [Starter template](modules/example/) — copy it into `data/plagin/example/`, reload addons, enable it and save.
- [PC statistics](addons/pcstats/) · [Notifications](addons/notifications/).
- [Creating radial menu themes](docs/FLOATING_THEMES.md).
- [Internal protocol and app formats](docs/PLUGIN_API.md).

### Practical limitations

Text input and automation depend on the active application's permissions and Windows UI Automation support; support for every field in every application is not guaranteed. Media keys control the active media session, while audio devices are selected through Windows Core Audio. Addons run with the current user's permissions. WebView2 uses Chromium; a fixed RAM footprint is not guaranteed.

### Source and credits

The [repository](https://github.com/MysterSay/Macro_Leprechaun) contains the Go desktop code, Android client, addon examples and documentation. Credits and licenses for the ported Kando components are listed in the [port notice](third_party/kando/PORT-NOTICE.md) and the project's license files.

[Support MacroLeprechaun](https://send.monobank.ua/jar/3rqmBsdneV)

---

## Українська

Ваша особиста автоматизація Windows: складайте макроси з пазлів і запускайте їх із телефона Android, віджета на краю екрана або плаваючого кругового меню.

[Сайт проєкту](https://macroleprechaun-web.sanya-1998-16-4.workers.dev/) · [API доповнень](docs/ADDON_API_V2.md) · [Підтримати розробку](https://send.monobank.ua/jar/3rqmBsdneV)

### Можливості

- **Три режими керування.** Смарт-панель на телефоні, віджет зі списком дій та кругове меню біля курсора. Режими можна вимикати окремо; вимкнення смарт-панелі зупиняє сервер керування телефоном.
- **Редактор пазлів.** Магнітні з’єднання, головна гілка від input, окремі true/false-гілки, групове виділення й переміщення. Неприєднані пазли не виконуються.
- **Текст.** Вставлення без буфера обміну, запис у буфер, заміни у виділеному тексті, виділення та пошук найближчого до курсора збігу.
- **Windows.** Команди в доступних терміналах, файли та програми з аргументами, папки, звук і мікрофон, браузерні команди, гарячі клавіші та клікер.
- **Дії.** Затримка, умови, очікування, цикли, паралельний запуск і зупинка макросів, читання й запис змінних. Цикли потрібно окремо дозволити в налаштуваннях; є кнопка зупинки всіх макросів.
- **Панелі.** Ієрархія з одним батьком, зміна панелі, перехід назад і до кореня у відповідному режимі керування.
- **Кнопки, перемикачі й повзунки.** Кольори, значки з файлів, фони, масштабування зображень, анімації та ефекти. Для повзунків — орієнтація й активація після зміни або під час зміни з інтервалом.
- **Сітка смарт-панелі.** Вільне розташування, розтягування у вільні клітинки, розділення кнопки на кілька зон і об’єднання вибраних кнопок з вибором макроса.
- **Живі підписи.** Використовуйте `${volume} ${system__time}`. Доступні власні змінні, системні час, дата й назва мережі та змінні доповнень.
- **Відкриті доповнення.** PowerShell, Python або JavaScript без компіляції; власна категорія пазлів, змінні й HTML-вікна налаштувань. Готові приклади: статистика ПК та сповіщення.
- **Оформлення кругового меню.** Рендерер і теми на основі Kando; відкриті стилі в `data/effects/floating/`.
- **Робота локально.** Телефон підключається до ПК через локальну мережу; для керування не потрібен хмарний обліковий запис. Макроси завантажуються з диска на вимогу, постійний кеш RAM необов’язковий; невикористані зовнішні змінні можна не опитувати.

### Завантаження і запуск

1. Завантажте `MacroLeprechaun.exe`; для телефона — `MacroLeprechaun.apk`.
2. Помістіть EXE у папку, доступну для запису, та запустіть. Програма створює й використовує `data` поруч з EXE. Якщо використовуєте повний пакет, зберігайте його структуру.
3. Для редактора і кругового меню потрібен **Microsoft Edge WebView2 Runtime**. Використовується окремий профіль програми, без особистих розширень браузера.
4. Для телефона ввімкніть смарт-панель, відкрийте «Налаштування → Підключити телефон» і створіть код підключення. ПК та телефон мають бути в одній доступній локальній мережі; дозвольте програмі доступ у брандмауері Windows для цієї мережі.
5. Створіть кнопку, відкрийте «Редагувати макрос», приєднайте пазли до input, застосуйте та збережіть зміни.

**Вимоги:** Windows 10/11 x64; Android 8.0 або новіший для APK. Телефон є пультом: дії виконує ПК, тому програма на ПК має працювати.

### Дані

| Каталог | Призначення |
| --- | --- |
| `data/macros/` | JSON-файли макросів |
| `data/setting/` | База SQLite налаштувань програми та кнопок, службові дані інтерфейсу |
| `data/icon/` | Файли значків |
| `data/image/` | Зображення для фонів |
| `data/plagin/` | Доповнення з відкритим кодом |
| `data/effects/floating/` | Теми плаваючої панелі |
| `data/locales/` | JSON-файли локалізацій |

Для резервної копії закрийте програму й скопіюйте `data`. Не додавайте власну робочу `data` з макросами, паруванням та налаштуваннями до публічного релізу. Діагностика запуску — `data/application.log`.

### Доповнення та теми

- **[Повний API доповнень](docs/ADDON_API_V2.md)** — маніфест, інтерпретатори, stdin/stdout, змінні, HTML-мости та межі доступу.
- [Готовий шаблон](modules/example/) — скопіюйте в `data/plagin/example/`, перечитайте доповнення, увімкніть і збережіть.
- [Статистика ПК](addons/pcstats/) · [Сповіщення](addons/notifications/).
- [Створення тем кругового меню](docs/FLOATING_THEMES.md).
- [Внутрішній протокол та формати програми](docs/PLUGIN_API.md).

### Практичні обмеження

Вставлення та автоматизація тексту залежать від прав активної програми й підтримки Windows UI Automation; довільне поле будь-якої програми не гарантується. Медіаклавіші керують активним медіасеансом, а звукові пристрої вибираються через Windows Core Audio. Доповнення виконуються з правами поточного користувача. WebView2 використовує Chromium; фіксоване споживання RAM не гарантується.

### Вихідний код і авторство

[Репозиторій](https://github.com/MysterSay/Macro_Leprechaun) містить Go-код ПК, Android-клієнт, приклади доповнень та документацію. Авторство й ліцензії перенесених компонентів Kando наведені в [повідомленні про перенесення](third_party/kando/PORT-NOTICE.md) та файлах ліцензій проєкту.

[Підтримати MacroLeprechaun](https://send.monobank.ua/jar/3rqmBsdneV)
