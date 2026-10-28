# MacroLeprechaun — Windows 0.17.1 / Android 0.5.0

Ваші дії. Ваші правила. MacroLeprechaun допомагає автоматизувати повторювані дії Windows і керувати ПК з телефона або прямо з робочого столу.

## Що вміє програма

- Три режими: смарт-панель Android, віджет на краю екрана та плаваюче кругове меню.
- Візуальне складання макросів із пазлів: з’єднання, input, true/false-гілки, умови, затримки, очікування та цикли.
- Робота з текстом, файлами, програмами, терміналами, браузером, клавішами, мишею, звуком і мікрофоном.
- Запуск і зупинка макросів, змінні, підписи на кшталт `${volume} ${system__time}` та переходи між панелями.
- Власний вигляд кнопок, перемикачі й повзунки; розтягування, розділення та об’єднання кнопок смарт-панелі.
- Відкриті скриптові доповнення без компіляції: власні пазли, змінні та HTML-налаштування. Є приклади статистики ПК і сповіщень.
- Теми кругового меню на основі Kando, локальні дані та завантаження макросів на вимогу.

## Файли релізу

| Файл | Для чого |
| --- | --- |
| `MacroLeprechaun.exe` | Програма для Windows 10/11 x64, версія 0.17.1 |
| `MacroLeprechaun.apk` | Пульт для Android 8.0+, версія 0.5.0 |

## Початок роботи

1. Завантажте EXE у папку з дозволом на запис і запустіть. Потрібен Microsoft Edge WebView2 Runtime.
2. Встановіть APK на телефон, якщо хочете керувати ПК з Android.
3. Підключіть пристрої до однієї локальної мережі, увімкніть смарт-панель і створіть код у «Налаштування → Підключити телефон».
4. Додайте кнопку, приєднайте пазли до input та збережіть налаштування. Неприєднані пазли не запускаються.

ПК має залишатися ввімкненим із запущеною програмою. Автоматизація текстових полів залежить від прав і підтримки UI Automation активною програмою. Цикли за замовчуванням вимкнені. Доповнення виконуються з правами користувача; встановлюйте код, якому довіряєте.

**Ваші дані:** папка `data` поруч з EXE. Перед заміною файлів закрийте програму й зробіть її резервну копію.

## Документація та підтримка

- [README](https://github.com/MysterSay/Macro_Leprechaun/blob/main/README.md)
- [API відкритих доповнень](https://github.com/MysterSay/Macro_Leprechaun/blob/main/docs/ADDON_API_V2.md)
- [Вихідний код](https://github.com/MysterSay/Macro_Leprechaun)
- [Підтримати розробку 💚](https://send.monobank.ua/jar/3rqmBsdneV)

---

## English

MacroLeprechaun is a Windows automation tool with an Android control panel, an edge widget and a radial menu. Build macros from connected puzzle blocks, automate text and Windows actions, use variables and conditions, and extend the app with open-source scripts and custom settings pages.

Download `MacroLeprechaun.exe` for Windows 10/11 x64 and `MacroLeprechaun.apk` for Android 8.0+. The desktop UI requires Microsoft Edge WebView2 Runtime. Keep the desktop app running and connect the phone through the same reachable local network. App data is stored in the `data` folder next to the EXE.

Windows package: **0.17.1**. Android package: **0.5.0**.
