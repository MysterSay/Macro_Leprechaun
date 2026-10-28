> Для створення нових відкритих доповнень використовуйте [API 2](ADDON_API_V2.md). Нижче — внутрішній протокол програми та історичні формати; вони не розширюють HTML-міст доповнення.

> Для нових доповнень використовуйте відкриті скрипти та `runtime` + `entry`: див. **ADDON_API_V2.md**. Нижче — довідник старого сумісного API. Приклад у modules/example тепер працює без компіляції.

# MacroLeprechaun API v1

## Модель розширення

Доповнення — окремий локальний процес. Підійде Go, Python зі своїм пакуванням, Java та інші мови. Немає залежності від Go ABI/DLL. Модулі виконуються з правами користувача; це не sandbox. Після встановлення потрібне явне увімкнення у Доповненнях та збереження налаштувань.

Папка: `<data>/plagin/<folder>/`. Обов’язковий manifest.json:

```json
{
  "id": "example",
  "name": "Example · Clock",
  "version": "1.0.0",
  "api": 1,
  "executable": "example.exe",
  "args": [],
  "actions": ["clock"]
}
```

- id: унікальний ASCII ідентифікатор, 1–80 символів A–Z, a–z, 0–9, `_`, `-`.
- api: рівно 1. Несумісні версії не запускаються.
- executable: відносний шлях усередині папки модуля. Абсолютні шляхи та `..` відхиляються. Не використовуйте симлінки назовні; папки модулів повинні бути довіреними.
- args: фіксований масив аргументів процесу, без shell-інтерпретації та без підстановки змінних. Для Python/Java покладіть власний інтерпретатор/runtime всередину модуля або зберіть EXE.
- actions: перелік дозволених імен дій. Крок module звертається до `id/action`.

**Виклик:** кожен крок створює новий процес. Робоча папка — папка виконуваного файла. На stdin передається один JSON-об’єкт із завершальним LF; потім stdin закривається:

```json
{"api":1,"action":"clock","variables":{"name":"Олександр","volume":"50"}}
```

На stdout поверніть рівно один JSON-об’єкт. Усі значення variables — рядки:

```json
{"ok":true,"message":"Clock updated","variables":{"clock":"21:45:00"}}
```

Помилка:

```json
{"ok":false,"message":"Пояснення помилки"}
```

Exit code має бути 0. Ненульовий exit code, неправильний JSON, ok=false або перевищення часу зупиняють макрос. Вихід обмежено 8192 байтами; stdout і stderr об’єднуються цією версією. Тому **не пишіть діагностичний текст у stdout або stderr**, використовуйте message або окремий файл модуля. message потрапляє в журнал. Повернені variables одразу доступні наступним крокам і спільному стану.

Час — залишок спільного 60-секундного бюджету макросу, не 60 секунд на кожний крок. Не запускайте фонові дочірні процеси. Постійні фонові модулі, події та push-підписки поки не підтримуються.

Приклад модуля: `modules/example/main.go`. Зібрати:

```powershell
go build -buildvcs=false -o example.exe main.go
```

Скопіювати manifest.json та example.exe в `build/data/plagin/example`, натиснути «Перечитати доповнення», увімкнути Example · Clock, зберегти. Додати до макросу крок module зі значенням `example/clock`. Після запуску `${clock}` можна використати у кроці text.

## Формат конфігурації

Зберігається у data/setting/settings.sqlite та data/macros/*.json (див. розділ 0.4.0 нижче). Редагуйте через інтерфейс або адміністративний API. Збереження перевіряється перед заміною файла. Поля:

```json
{
 "settings":{"language":"uk","theme":"dark","accent":"#6dde91","port":47640},
 "smart":[],"widgets":[],"floating":[],
 "macros":[],"variables":{},"modules":{}
}
```

Макрос:

```json
{
 "id":"greeting","name":"Привітання","icon":"✎",
 "steps":[
  {"type":"delay","value":"","ms":500},
  {"type":"text","value":"Привіт, ${name}!"}
 ]
}
```

| type | value | args / ms |
|---|---|---|
| launch | EXE або шлях | args: масив окремих аргументів; запускається без shell |
| shell | буквальний PowerShell-скрипт | args не застосовуються |
| keys | CTRL+SHIFT+S або MEDIA_PLAY_PAUSE | див. README для переліку клавіш |
| text | Unicode-текст | введення в активне вікно |
| delay | порожній рядок | ms: 0–60000 |
| variable | назва змінної | args: ["нове значення"] |
| module | example/clock | змінні передаються через stdin |

Підстановка `${key}` у value та args — буквальна заміна рядка. Невідомі імена залишаються без змін. Порядок вкладених підстановок не гарантується; не використовуйте змінні, що посилаються одна на одну. Вибір shell означає виконання вашого скрипту з правами користувача.

Панель:

```json
{
 "id":"main","name":"Головна","enabled":true,"columns":3,
 "edge":"right","monitor":0,"position":50,"thickness":4,
 "hotkey":"CTRL+ALT+SPACE","radius":160,
 "controls":[
  {"id":"hello","label":"Привіт","icon":"✎","kind":"button",
   "macro":"greeting","offMacro":"","variable":"","color":"#6dde91"}
 ]
}
```

smart/widgets/floating незалежні. ID панелі унікальний у своєму режимі, ID контролу — у панелі. Macros — спільні, ID унікальний. Ліміти: 500 макросів, 100 кроків/макрос; 30 smart-панелей, 20 widgets, 20 floating; 48 контролів/панель, для floating максимум 12. Віджети/радіальні меню підтримують тільки kind=button. Smart: button, toggle, slider, input. Для slider/input потрібна variable. При toggle=false використовується offMacro, якщо вказано; інакше macro. Сервер не зберігає стан прапорця; телефон зберігає його до перезавантаження сторінки. Повзунок надсилає значення лише після завершення зміни, поле — після підтвердження/втрати фокусу.

Колір інтерфейсу — #RRGGBB. Theme: dark/light. Language — безпечне ім’я JSON-словника. Monitor — індекс Win32 EnumDisplayMonitors, 0 — перший; недоступний індекс використовує найближчий монітор. Для глобального hotkey використовуються RegisterHotKey і MOD_NOREPEAT; конфлікти з іншими програмами видимі у журналі. Стандартні зарезервовані Windows-комбінації можуть бути недоступні.

## HTTP для клієнтів

База: `http://<PC-IP>:47640`. JSON UTF-8. Тіло запиту максимум 2 MiB. Сервер не має CORS. Чужий Origin у змінювальних запитах відхиляється. Android і вебпанель використовують same-origin.

### Виявлення

- UDP broadcast `255.255.255.255:47641`, тіло `MACROLEPRECHAUN_DISCOVER_V1` (ASCII, без LF).
- Відповідь unicast: `{"name":"MacroLeprechaun","port":47640}`. Адресу сервера беріть з UDP-відправника.
- mDNS: `_macroleprechaun._tcp.local.`, hostname `macroleprechaun.local.`. Пряме IP-підключення — резервний шлях.
- GET `/api/discover`, без токена: `{"name":"MacroLeprechaun","version":"0.7.0","port":47640}`.

### Спарювання

На ПК створюється одноразовий шестизначний код, строк 5 хвилин. До 5 невдалих спроб з однієї IP-адреси, потім потрібен новий код.

POST `/api/pair` без авторизації:

```json
{"code":"123456","name":"Мій Android"}
```

200: `{"token":"<48 hex characters>"}`. Зберігайте токен приватно, надалі заголовок `Authorization: Bearer <token>`. Помилка/прострочення — 403. Після відкликання — 401.

### Панелі та виконання

GET `/api/panels` із токеном повертає `{panels:[...],settings:{...},variables:{...}}`. Тільки enabled smart-панелі. Немає визначень команд, списку доповнень або адміністративного ключа. Змінні вважаються спільними з телефоном; не зберігайте у них паролі чи інші секрети.

POST `/api/control`:

```json
{"panel":"main","control":"hello","value":""}
```

Телефон викликає лише контролі з активних smart-панелей. Не може передати довільний shell/ID макросу. Для полів значення до 4096 символів записується в налаштовану variable та передається запуску. Для перемикача value — рядок `"true"` або `"false"`. 200 означає прийняття запуску, не завершення макросу. Результат виконання — у журналі ПК. 404 — невідомий/відключений контрол, 409 — макрос уже активний або 8 зайнятих слотів.

### Адміністративний API

Локальний одноразовий URL головного вікна створюється сервером із ключем сесії процесу. Ключ обмінюється на HttpOnly SameSite=Strict cookie тільки через loopback. Не передавайте його телефону. Headless-режим `-headless` друкує URL у локальну консоль. Після перезапуску потрібне нове вікно/новий URL, токени телефонів лишаються чинними.

| Метод | URL | Тіло / результат |
|---|---|---|
| GET | /api/config | повна конфігурація |
| POST | /api/config/validate | повна конфігурація; перевірка без запису, запуску чи зміни стану |
| PUT | /api/config | повна конфігурація; перевірка + атомарний запис |
| GET | /api/status | logs, running, devices, pairCode, pairUntil, modules, addresses |
| POST | /api/pairing | `{}`; новий код і скидання ліміту спроб |
| POST | /api/revoke | `{"id":"device token"}` |
| POST | /api/run | `{"id":"macro ID"}` |
| POST | /api/modules/reload | `{}`; повторне сканування manifest.json |
| POST | /api/quit | `{}`; завершення сервера та нативних вікон |

Успіх змінювальних операцій — `{"ok":true}`. Помилки — звичайний UTF-8 текст, HTTP 400/401/403/404/405/409/500. Обробляйте HTTP-статус перед JSON. Адміністративний API не є публічним віддаленим API. Для збереження зовнішньої сумісності використовуйте api=1 у маніфесті; майбутні зміни контракту матимуть інший номер.


## JSON-профілі редактора (0.2.0)

Експорт у Налаштуваннях створює файл:

```json
{
 "format":"macroleprechaun-profile",
 "version":1,
 "createdAt":"2026-10-27T20:00:00.000Z",
 "config":{"settings":{},"smart":[],"widgets":[],"floating":[],"macros":[],"variables":{},"modules":{}}
}
```

config має відповідати повному контракту конфігурації вище; settings у прикладі скорочено. Масиви не можуть бути null: smart, widgets, floating, macros, steps та controls мають бути JSON-масивами. Конфігурація не містить токенів пристроїв чи ключа адміністратора. variables можуть містити особисті дані — експорт переносить їх буквально.

Імпорт приймає format=macroleprechaun-profile, version=1, максимум 2 MiB. Спочатку POST /api/config/validate, далі підтвердження завантаження в редактор; чинний сервер змінюється лише при окремому PUT /api/config. Редактор не виконує імпортованих макросів автоматично. Копіювання панелі не копіює макроси — лише контролі з новими ID, які посилаються на чинні макроси. Копіювання макросу створює новий ID і незалежну копію його кроків.


## Інтерфейс 0.3.0

Формат Config/Panel/Control сумісний із 0.2.0. columns впливає на Smart-панель; віджет візуально завжди список. Плаваюче меню використовує angular-sector hit testing: індекс рахується за кутом від 12 години за годинниковою стрілкою, центр радіусом 52 px скасовує вибір. Дистанція до конкретної кнопки не обмежує активацію. Натискання миші перехоплюються лише поки плаваюче меню відкрите.

Редактор контролу створює незалежний макрос при зміні кроків або виборі шаблону; references у вже наявних контролів залишаються без змін. Назви/іконки користувача та його макроси не перекладаються автоматично. Макроси, що не використовуються, залишаються у списку шаблонів.

Візуальна довідка: Kando default theme, https://github.com/kando-menu/kando/blob/main/assets/menu-themes/default/theme.css (Simon Schneegans, CC0-1.0). Використано її геометрію як зразок; рендеринг і обробка подій тут власні, на Go/Win32, не движок Kando.

## 0.4.0: пазли, клітинки та дискове зберігання

Macro отримав необов’язковий puzzles:

```json
{"id":"draft","name":"Приклад","icon":"✦","steps":[],"puzzles":[
 {"id":"piece1","type":"text.insert","category":"text","value":"Привіт","x":96,"y":48},
 {"id":"piece2","type":"actions.delay","category":"actions","value":"500","x":96,"y":164}
]}
```

puzzles — максимум 100. ID унікальні в макросі; X/Y 0–10000; value — до 16384 символів. Координати задають візуальне положення; при збереженні редактор упорядковує за Y, потім X. Це **не граф виконання**. Макрос із непорожнім puzzles не може виконуватися: /api/run або виклик контролу повертає 409, жоден step не запускається. Умови, запуск інших макросів, папки та всі нові типи — лише прототипи пазлів.

Control отримав cell (індекс від нуля в рядковому порядку) і appearance:

```json
{"id":"b","label":"Приклад","icon":"/media/icon/logo.png","kind":"button","macro":"draft","color":"#6dde91","cell":8,
 "appearance":{"text":"#ffffff","border":"#68dd90","background":"#202020","animation":"pulse","effect":"glow","press":"ripple"}}
```

Без cell старі контролі розташовуються за індексом у масиві. Cell 0–239, значення у панелі не можуть повторюватися. panel.rows 1–30, panel.background — ім’я файла з data/image. Порожні клітинки зберігаються; drag на зайняту клітинку міняє два cell місцями. Appearance: animation none/pulse/spin/float, effect none/glow/shadow/glass, press none/scale/flash/ripple. Кольори #RRGGBB. Порожні значення використовують тему.

Manifest доповнення може оголошувати власні категорії, незалежно від actions:

```json
{"id":"example","name":"Example","version":"1.0.0","api":1,"executable":"example.exe","args":[],"actions":["clock"],
 "puzzleCategories":[{"id":"clock","name":"Годинник","blocks":[{"id":"now","name":"Поточний час"}]}]}
```

Редактор будує type `plugin:example/clock/now`. Категорії з’являються для увімкнених доповнень після перечитування маніфестів/збереження. Вони не запускають виконуваний файл і поки мають тільки текстовий параметр. Не плутайте прототип puzzle-блоку з виконуваним module-step попереднього API.

Дані тепер у data/macros/*.json та справжній SQLite data/setting/settings.sqlite. SQLite зберігає налаштування програми, панелей, кнопок та індекс JSON-файлів макросів. Див. data/setting/schema.sql. Структура HTTP Config лишається повною, тому формат експорту/імпорту version=1 збережено. Токени пристроїв — data/setting/devices.json; доповнення — data/plagin. Старі config.json та data/modules мігрують при першому запуску; README описує резервні копії.

GET /api/status додатково повертає media: {icon:[filename...],image:[filename...]}. GET /media/icon/filename та /media/image/filename віддають локальні PNG/JPG/JPEG/GIF/WebP без можливості виходу за каталог. Не кладіть приватні документи в медіапапки: ці зображення доступні клієнтам локальної мережі.


## 0.4.1: масштабування телефона

Формат даних незмінний. Усі rows × columns поточної панелі показуються одночасно, включно з порожніми клітинками. Розмір вмісту пропорційно адаптується до клітинок. Пагінації й стрілок між панелями немає; показується перша активна Smart-панель. Навігація через папки й пазли ще не реалізована. Кількість рядків змінюється тільки в редагуванні панелі, фон — у галереї прев’ю.


## 0.5.0: значки, фон та ефекти

Control.appearance.image — необов’язковий локальний шлях `/media/image/name.png` до фону кнопки. Control.icon підтримує текстовий символ або `/media/icon/name.svg` та растрові формати. Імпортовані SVG перевіряються; вони подаються через img із обмежувальним CSP, не вставляються як HTML. Маніпуляції файлами/мережею — лише адміністративні API:

- GET /api/icons/search?source=iconify|simpleicons|devicon|svgl|vectorlogo&q=query → [{name,url}]. VectorLogoZone приймає точний slug і формує кандидат URL, доступність якого перевіряється під час прев’ю/імпорту.
- GET /api/icons/preview?url=... → SVG-прев’ю з підтримуваного джерела; потрібна авторизація адміністратора.
- POST /api/media/import: {kind:"icon"|"image",url:"https://…"} або {kind:"icon"|"image",data:"base64 bytes"} → {path,name}. Файл зберігається за хешем. Для URL джерело записується в sidecar .source.json.

URL імпорту обмежені HTTPS-вузлами джерел та офіційними CDN/raw GitHub-шляхами. Перенаправлення перевіряються повторно. SVG зі script, on*-обробниками, foreignObject або зовнішніми href відхиляється. HTTP-запит максимум 2 MiB, тому base64-файли повинні бути меншими; більші зображення можна покласти до media-папки без HTTP-імпорту.

Control.variable залишається застарілим необов’язковим полем, але більше не використовується /api/control; значення від поля/повзунка не записується у Config.variables. Їхні значення поки локальний стан елемента керування; майбутня передача у пазли не реалізована. Пазли як і раніше не виконуються.

Джерела API: https://iconify.design/docs/api/svg.html та https://svgl.app/docs/api. Файли мають власні умови використання джерел; URL джерела зберігається поряд із завантаженим значком.


## 0.6.0: граф input та тригери

Нові макроси мають graphVersion=1. Рівно один puzzle з type=input обов’язковий:

```json
{"id":"toggle","name":"Toggle","icon":"✦","steps":[],"graphVersion":1,"puzzles":[
 {"id":"start","type":"input","category":"input","value":"","x":96,"y":48,"mode":"toggle","trueNext":"on","falseNext":"off"},
 {"id":"on","type":"text.insert","category":"text","value":"on","x":96,"y":208,"next":"next"},
 {"id":"next","type":"actions.delay","category":"actions","value":"500","x":96,"y":324},
 {"id":"off","type":"text.insert","category":"text","value":"off","x":436,"y":208},
 {"id":"loose","type":"win.command","category":"win","value":"ignored","x":900,"y":400}
]}
```

input.mode: button/toggle/slider. Button і slider використовують next, toggle — trueNext/falseNext. Інші блоки мають один next. Посилання за ID, не за координатами. Неіснуючі цілі, посилання на input, кілька батьків, цикли та дублікати ID заборонені. Неприєднані блоки зберігаються, але не відвідуються. Старі макроси без graphVersion лишаються сумісними; для puzzle-чернеток input додається у редакторі. Відомий порядок legacy steps конвертується у ланцюжок; координати старих вільних пазлів не вважаються зв’язками.

Slider input додатково має max (0 < max <= 1e9) і variable (ідентифікатор змінної). Приклад: {"id":"start","type":"input","mode":"slider","max":250,"variable":"volume","next":"node","x":96,"y":48,"value":""}. Клієнт надсилає числовий рядок value; сервер перевіряє діапазон 0..max, записує Config.variables[variable] перед проходженням гілки. У toggle передаються рядки true/false, у button — click. offMacro для graphVersion=1 ігнорується: обидва виходи належать одному графу.

У 0.6.0 виконуються тільки вхід/маршрутизація і запис значення slider. Інші пазли відвідуються у порядку next та логуються як заготовки, без виконання функцій. /api/control повертає прийняття запуску; повний шлях видно в logs. /api/run без події підтримує button input; toggle/slider викликайте через налаштований контрол.

GET /api/panels додатково повертає inputs: {"panel/control":{"max":250}} і states: {"panel/control":{"active":true,"value":0}}. Стан зберігається у процесі сервера; перезавантаження сторінки його не скидає, перезапуск ПК/сервера скидає. Значення змінних зберігаються на диск під час збереження конфігурації, як і раніше.

Control.appearance.activation — необов’язкова кількість секунд візуального натиснення (0–60, типово 0.22). Не впливає на тривалість макросу. Тип input (текстове поле) більше не пропонується; старі записи читаються для сумісності та показуються як кнопки.


## 0.7.0: текстові дії

Текстові пазли виконуються у приєднаній гілці. `value` містить буквальний текст без підстановки змінних. `text.insert` вводить у фокусоване поле без буфера (EM_REPLACESEL для Windows Edit, SendInput для інших полів); `text.clipboard` лише записує CF_UNICODETEXT. `text.transform` приймає список `джерело - заміна`, один рядок на правило. Це точна регістрозалежна заміна підрядків у виділеному тексті, без каскаду. Перший збіг у порядку правил має пріоритет. `text.select.value`: `all`, `sentence`, `line`, `before`, `after`. Порожнє значення означає `all`. `text.find.value`: буквальний пошуковий рядок.

Трансформація/виділення/пошук доступні лише для стандартного Windows Edit, не password, до 60000 UTF-16 одиниць. Інші редактори не модифікуються: помилка в logs зупиняє гілку. Успішний /api/control підтверджує прийняття запуску, а не результат кожної асинхронної дії. Нетекстові пазли залишаються заготовками. Під час виконання текстові графи серіалізуються, щоб уникнути перемішування вводу.


## 0.7.1: адаптер UI Automation і діагностика

Для нестандартних полів text.select/find/transform використовують Windows UI Automation TextPattern, не буфер обміну. Це вимагає підтримки провайдера активного поля; помилки доступні у logs та application.log. Читання виділення до 16384 UTF-16 одиниць, сканування line/sentence до 16384 кроків, таймаут 15 s. Системний PowerShell запускається приховано у STA, команда вбудована в EXE, параметри передаються JSON через stdin. /api/status містить version. text.insert використовує пакетний SendInput; переноси — Shift+Enter. Дані clipboard-пазла записуються як раніше. Не гарантуйте підтримку довільних полів лише за назвою браузера/програми.


## 0.7.2: повне виділення та найближчий пошук

text.transform зберігає всі символи виділеного фрагмента, для яких немає правил заміни. UIA перед записом повторно перевіряє весь фрагмент. ValuePattern застосовується лише при точному зіставленні повного тексту поля; інакше повторно виділяється оригінальний діапазон для Unicode-введення. Сусідній текст зберігається; clipboard не використовується.

text.find шукає найближчий збіг до правої межі поточного виділення/курсора з обох боків, а не тільки вперед. Відстань рахується до найближчої межі збігу у UTF-16 одиницях; всередині збігу — 0. За нічиєї обирається збіг на/після курсора. UIA: до 10000 збігів, 15 s timeout. Формат JSON пазлів не змінено.


## 0.8.0: Windows-пазли

Win-пазли тепер виконуються лише у приєднаній гілці. `Puzzle.value` містить JSON-рядок з налаштуваннями:

| type | Поля JSON |
|---|---|
| win.command | terminal (powershell/pwsh/cmd/python/py/wsl:<дистрибутив>), command, directory, output |
| win.file | path, app (порожнє = асоціація Windows) |
| win.folder | path |
| win.program | path (EXE або shell:AppsFolder\AppID), args |
| win.sound | action, device (ID Core Audio або default), steps, variable |
| win.browser | action (open/refresh/home/back/forward), url |
| win.hotkeys | keys (масив Windows-кодів із меню) |
| win.clicker | x, y, buttons (right/middle/left/x1/x2) |

Sound action: next/previous/playpause/stop (медіаклавіші, device не застосовується), mute/down/up/set, mic_mute/mic_down/mic_up/mic_set. steps: 1–100 процентних пунктів; set читає variable як число 0–100. device — endpoint, не аудіосесія окремої програми. Власні теги X-Mouse не підтримуються.

Адміністратор, лише loopback: GET /api/win/inventory → {terminals:[{id,name,path}], apps:[{name,path}], devices:[{id,name,flow:render|capture}], audioError, platform}; POST /api/win/pick {kind:file|folder|program|output} → {path}; GET /api/win/cursor → {x,y,space}. Скасований picker повертає порожній path. Телефон доступу до цих endpoint не має. Решта OS/error-поведінки та межі описані у README.

Редактор повторно використовує ID макроса при звичайному редагуванні; список не є журналом версій. Історичні неприєднані до контролів пазлові макроси прибираються при завантаженні редактора; зміна персистується через Save. Явний duplicate створює новий ID. JSON сховище зберігає лише поточний індекс.


## 0.8.1: inventory та очищення

Виправлено IID колекції Core Audio. Непорожній audioError з /api/win/inventory залишається видимим при зміні звукової дії і записується до application.log; повторний GET через кнопку refresh оновлює список. При відсутності активних endpoint відповідного flow UI показує окреме повідомлення.

pruneMacroHistory зберігає лише макроси, досяжні від macro/offMacro контролів усіх трьох режимів і приєднаних actions.macro. Старі steps-приклади без прив’язок більше не мають винятку. Зміни персистуються звичайним Save; сховище прибирає власні застарілі JSON після успішного commit.


## 0.8.2: missingFile

Macro має необов’язковий `missingFile:string`: файл був у SQLite macro_files, але відсутній на диску. Такий запис містить ID, name, icon, steps:[], graphVersion:0 без пазлів і не виконується. Поки користувач не вирішить, що робити, writer залишає індекс цього файла без створення JSON. Редактор не видаляє цей запис звичайним prune. Recreate замінює запис нормальним макросом з тим самим ID; delete прибирає запис та очищає його прив’язки. Порожній macro у контролі тепер означає «дія не призначена», а натискання не запускає макрос. Всі зміни персистуються звичайним Save. /api/config та export/import зберігають missingFile.


## 0.9.0: app actions, parent і activePanel

Panel.parent: ID батьківської панелі у тому самому режимі або порожній рядок. Валідація забороняє невідомого батька й цикли. GET /api/panels додатково повертає activePanel (ID активної панелі цього device-token). Навігація має історію на процес, окрему для кожного токена/режиму/початкового нативного віджета. Панелі інших режимів не відкриваються у телефоні.

Puzzle.value для нових типів — JSON-рядок:

| type | Поля |
|---|---|
| folders.change | panel:ID |
| folders.parent / folders.root | налаштування не потрібні |
| actions.delay | milliseconds:int64, 0..31622399999 |
| actions.condition / actions.wait | condition:{operator,first,second} |
| actions.macro | macro:ID |
| actions.set | variable, source:selection/clipboard/file/static, path, value |
| actions.get | variable, destination:field/clipboard/file, path |

Операнд condition: {variable:"left",value:"literal",valueVariable:"right"}; якщо valueVariable непорожнє, value ігнорується. first потрібен завжди; second — лише and/not/or. if=A, else=!A, and=A&&B, not=!A&&!B, or=A||B. Хибна condition зупиняє гілку. wait читає поточний cfg.variables кожні 100 мс без блокування інших дій; відсутня змінна завершує запуск помилкою. Звичайний 60-секундний timeout старих steps до graph-макросів більше не застосовується. При виході з процесу очікування припиняється.

actions.macro запускає незалежний runScoped із переданим контекстом панелі та ancestry. До 8 активних ID, не більш як 16 рівнів, заборона рекурсії. input: click/true/поточне числове значення slider-variable. prune враховує JSON macro-посилання лише приєднаних пазлів. Runtime-змінні не персистуються окремою транзакцією автоматично. Документи actions.set: до 1 MiB UTF-8/UTF-16 BOM, actions.get: UTF-8 з перезаписом. Подробиці в README.

/api/status (admin) додатково повертає variables — актуальний знімок runtime-значень. Редактор синхронізує лише локально незмінені значення; ручні правки залишаються пріоритетними. /api/run використовує smart-контекст поточної admin-сесії.


## 0.10.0

Див. ADDON_API_V2.md щодо реального виконання plugin:<module>/<category>/<block>, UI та змінних. Settings.allowLoops:boolean — default false. Puzzle.body:string — друга гілка лише actions.loop; next — продовження після виконання умови. actions.loop value:{condition,interval}, interval мілісекунди >=10. actions.stop value:{all:boolean,macros:[ID,...]}. POST /api/stop {all:boolean,ids:[ID,...]} — admin, скасувати активні запуски. GET /api/locales — admin, список доступних кодів. GET /api/plugin/ui?module=ID&page=declared.html — admin, sandbox/CSP сторінки UI. Config.moduleSettings[ID] — довільний JSON-об’єкт налаштувань API 2. Module.runtimeError — остання помилка фонових змінних.

variablesSnapshot/config/status включають readonly system__time/date/network та namespace-значення доповнень. save не персистує ці знімки як користувацькі змінні. Імена з __ зарезервовано. Пазл запису/slider не може писати у readonly-простір. Системні дані часу беруться з ПК, а network — імена підключених профілів Windows. folders.* тепер виконується лише у smart/телефонному контексті; для нативних панелей повідомляється помилка без переходу.
