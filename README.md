# umeshprep — утилита для построения исходного дерева Umesh из wasmd

`umeshprep` — консольная утилита, которая **на хосте** (не внутри контейнера ноды)
выводит готовое исходное дерево Umesh, основанное на
[wasmd](https://github.com/CosmWasm/wasmd) и адаптированное под **Cosmos SDK
v0.54 + CometBFT**. Она зеркалирует скрипт
`scripts/common/prepare-umesh-from-wasmd.sh`, но переписан как
standalone-утилита на Go (модуль
`github.com/umesh-network/umeshctl/umeshprep`) без внешних shell-зависимостей.

Задачи `umeshprep`:

- клонировать wasmd на зафиксированной версии (shallow clone, `--depth 1`);
- переименовать Go-модуль (`github.com/CosmWasm/wasmd` → ваш форк, по умолчанию
  `github.com/umesh-network/umesh`) и бинарник (`wasmd` → `umeshd`);
- применить патчи под SDK v0.54 посредством **Go AST-правок** (а не regex/sed) к
  `app/app.go`: удаление deprecated-модулей (`group`, `nft`, `crisis`), условное
  удаление IBC/upgrade/feegrant/authz/vesting/protocolpool, фикс порядка
  `EndBlockers` (`x/bank` первым) и жёсткая подстановка `Wasm Capabilities`;
- поправить `go.mod` (пины `cosmos-sdk`/`cometbft` через `replace` + `go get` +
  `go mod tidy`);
- финализировать дерево: вычистить `contrib`/`docs`/`testing` и `_test.go` (если
  не `-keep-tests`), написать маркер `.wasmd-baseline`, пробить `go build` gate
  (`CGO_ENABLED=1`, т.к. wasmvm требует CGO) и **создать заново чистый git-репо**
  в `OUTPUT_DIR` с единственным коммитом `feat: initialize Umesh from wasmd <версия>`
  от имени `Umesh Bot`.

Этот документ — «переработанная» README для репозитория `umesh-prep`. Структура и
объяснения выстроены так, чтобы последовательно читать сверху вниз: что происходит
на каждом этапе, зачем он нужен и откуда берутся значения. Содержание проверено
по исходному коду (`main.go`, `rename.go`, `patch.go`, `patch_modules.go`,
`gomod.go`, `finalize.go`, `modules.go`, `exec.go`).

> **Важно:** `umeshprep` — это **только** генератор исходников. Сам Docker-образ
> `umesh-node`, `Dockerfile` и `docker-compose.yml` живут **в отдельном репозитории
> `Node_Umesh`** — `umeshprep` их не собирает и не включает.

## Содержание

1. [Требования и установка](#1-требования-и-установка)
2. [Конфигурация и секреты](#2-конфигурация-и-секреты)
3. [Быстрый старт](#3-быстрый-старт)
4. [Этапы работы утилиты](#4-этапы-работы-утилиты)
5. [Переключение модулей (module toggling)](#5-переключение-модулей-module-toggling)
6. [Что именно делает патч app.go (AST)](#6-что-именно-делает-патч-appgo-ast)
7. [Частые ошибки и диагностика](#7-частые-ошибки-и-диагностика)
8. [Разработка и тесты](#8-разработка-и-тесты)

## 1. Требования и установка

- **Go 1.25.0** или новее — для сборки `umeshprep` из исходников.
- **Docker** — нужен, только если вы собираете на хосте, где `go` отсутствует
  (`stepGomod` тогда прогоняет `go mod tidy` внутри `golang:1.25-alpine`;
  флагом `-in-container` можно всё время использовать нативный Go).
- **Сеть** — утилита клонирует wasmd из GitHub и ходит за модулями через
  `GOPROXY`.

```bash
git clone git@github.com:umesh-network/umeshctl.git   # или opscores/umesh-prep
cd umeshctl/umeshprep
go build -o umeshprep .
```

Проверка сборки:

```bash
./umeshprep -h
```

## 2. Конфигурация и секреты

Все параметры задаются единообразно: **флаг > env > default**. Приоритет
реализован через `getEnv(key, def)`-дефолты в `flag.NewFlagSet` — значит флаг,
переданный явно, всегда побеждает переменную окружения. Позиционных аргументов
нет (их наличие — ошибка).

| Параметр | Флаг | Env | По умолчанию | Описание |
| -------- | ---- | --- | ------------ | -------- |
| Версия wasmd | `-wasmd-version` | `WASMD_VERSION` | `v0.70.3` | Тег wasmd для `git clone --branch` |
| Репо wasmd | `-wasmd-repo` | `WASMD_REPO` | `https://github.com/CosmWasm/wasmd.git` | URL wasmd |
| Каталог вывода | `-output-dir` | `OUTPUT_DIR` | `./src` | Куда сложить итоговое дерево |
| Целевой модуль | `-target-module` | `TARGET_MODULE` | `github.com/umesh-network/umesh` | Новый путь Go-модуля |
| Bech32-префикс | `-bech32-prefix` | `BECH32_PREFIX` | `umesh` | Префикс адресов (bech32) |
| Home-каталог | `-node-dir` | `NODE_DIR` | `.umeshd` | Имя домашнего каталога ноды |
| Имя бинаря | `-binary-name` | `BINARY_NAME` | `umeshd` | Результирующее имя бинаря |
| version.Name | `-version-name` | `VERSION_NAME` | `umesh` | Значение `version.Name` в Makefile |
| Версия SDK | `-sdk-version` | `SDK_VERSION` | `v0.54.3` | Пин `cosmos-sdk` в `replace`/`go get` (только v0.54.x) |
| Версия CometBFT | `-cometbft-version` | `COMETBFT_VERSION` | `v0.39.3` | Пин `cometbft` в `replace`/`go get` |
| Capabilities (CSV) | `-capabilities` | `CAPABILITIES` | `iterator,staking,stargate,ibc2,cosmwasm_1_1,cosmwasm_1_2,cosmwasm_1_3,cosmwasm_1_4,cosmwasm_2_0,cosmwasm_2_1,cosmwasm_2_2,cosmwasm_3_0` | Жёсткий набор wasm-возможностей |
| GOPROXY | `-goproxy` | `GOPROXY` | `https://proxy.golang.org,direct` | `GOPROXY` для `go` команд |
| Тесты | `-keep-tests` | `KEEP_TESTS` | `false` (`"true"`) | Оставить `*_test.go` (иначе удалить в finalize) |
| Tidy | `-skip-tidy` | `SKIP_TIDY` | `false` (`"1"`) | Не выполнять `go mod tidy/go get` |
| Build gate | `-skip-build` | `SKIP_BUILD` | `false` (`"1"`) | Не выполнять `go build ./...` в finalize |
| In-container | `-in-container` | `UMESHPREP_IN_CONTAINER` | `false` (`"1"`) | Считать, что `go` есть на хосте |

### Флаги модулей

Модули делятся на 4 группы (см. `modules.go`):

| Группа | Модули | Флаг | По умолчанию | Примечание |
| -------| ------ | ---- | ------------ | ---------- |
| `core` (всегда включены) | auth, bank, consensus, staking, mint, distribution, slashing, gov, genutil, wasm, **evidence** | — | — | не выключаются |
| `ibc` | ibc, transfer, ica, ibctm | `-enable-ibc` (`ENABLE_IBC`) | `true` | при `false` удаляется `ibc2` из capabilities |
| `optional` | upgrade, feegrant, authz, vesting, protocolpool | `-enable-upgrade`/`-enable-feegrant`/`-enable-authz`/`-enable-vesting`/`-enable-protocolpool` (`ENABLE_*`) | `true`/`true`/`true`/`true`/`false` | protocolpool включить опционально |
| `deprecated` | group, nft, crisis | `-enable-deprecated` (`ENABLE_DEPRECATED`) | `false` | выключены, пока не включено вручную |

> `-enable-evidence` существует для обратной совместимости, но **это no-op**:
> `evidence` — core-модуль и всегда включён.

### Фичи (feature flags) — опционально, по умолчанию `false`

| Флаг | Env | Действие |
| ---- | --- | -------- |
| `-enable-blockstm` | `ENABLE_BLOCKSTM` | Block-STM параллельное исполнение (`patchBlockSTM`) |
| `-enable-iavlx` | `ENABLE_IAVLX` | IAVLx store optimization |
| `-enable-poa` | `ENABLE_POA` | Proof-of-Authority вместо staking (удаляет `staking`, требует ручной интеграции PoA) |
| `-enable-epochs` | `ENABLE_EPOCHS` | Модуль `x/epochs` (импорт, keeper, регистрация, BeginBlockers, genesis-порядок) |

Секретов, кроме `GOPROXY`, утилита не просит: всё публично (версии, URL, пути).

## 3. Быстрый старт

Самый простой сценарий — собрать Umesh из wasmd по умолчанию в `./src`:

```bash
umeshprep
```

Эквивалентно:

```bash
umeshprep -output-dir ./src -wasmd-version v0.70.3 \
  -target-module github.com/umesh-network/umesh -bech32-prefix umesh \
  -binary-name umeshd -sdk-version v0.54.3 -cometbft-version v0.39.3
```

С кастомной папкой и выключенной IBC:

```bash
umeshprep -output-dir ./umesh-tree -enable-ibc=false -skip-build
```

После успеха в `./umesh-tree` лежит чистый git-репо с одним коммитом
`feat: initialize Umesh from wasmd v0.70.3` (автор `Umesh Bot`), и внутри него —
готовое к сборке дерево Umesh:

```bash
cd ./umesh-tree
go build ./...        # CGO_ENABLED=1 не забыть перед production-билдом
```

## 4. Этапы работы утилиты

`umeshprep` выполняет 5 шагов последовательно (именно такой порядок в
`main.go`). Если любой шаг падает — работа завершается (`fatal`), а `OUTPUT_DIR`
оставляется в промежуточном состоянии (удобно для отладки).

| № | Шаг | Функция | Что делает |
|---|-----|---------|-----------|
| 1 | `clone wasmd` | `stepClone` | `git clone --branch <версия> --depth 1 <repo> <OUTPUT_DIR>`; **затирает** существующий `OUTPUT_DIR`; удаляет `.git` источника (история wasmd не нужна — финал дерева получает свой коммит) |
| 2 | `rename module and binary` | `stepRename` | `cmd/<binary>` (wasmd→umeshd), замена пути модуля во всех `.go` и в `go.mod`, переименование литералов `AppName`/`DefaultNodeHome`/`Bech32Prefix`/`NodeDir`, попытка вытащить `appName` из `binaryName` (`umeshd`→`UmeshApp`) |
| 3 | `patch app for SDK v0.54` | `stepPatch` | AST-пач `app/app.go`: удаление deprecated/опциональных модулей, перестановка `EndBlockers` (bank первым), подмена `wasmkeeper.BuiltInCapabilities()` на хардкодированный набор capabilities |
| 4 | `adjust go.mod` | `stepGomod` | `replace`-пины `cometbft`/`cosmos-sdk` + `go get`/`go mod tidy`; native `go` если на хосте есть, иначе через `docker run golang:1.25-alpine` с `--network=host -u <uid>:<gid>` |
| 5 | `finalize` | `stepFinalize` | удаление `contrib`/`docs`/`testing` и `_test.go` (если `!keep-tests`); метка `.wasmd-baseline`; build gate `CGO_ENABLED=1 go build ./...` (если `!skip-build`); `git init -b main` + коммит `feat: initialize Umesh from wasmd <версия>` как `Umesh Bot` |

> **Почему `rm -rf .git` после клона.** История wasmd нам не нужна — финал
> дерева получает **свою** чистую историю одним коммитом (см. шаг 5). Это избавляет
> от upstream-истории, `contrib` и `docs` wasmd.

## 5. Переключение модулей (module toggling)

Утилита проверяет согласованность выбранных модулей (`ValidateModules`): каждый
модуль должен иметь включённые зависимости. Например, `ibc` требует `evidence`,
`transfer` требует `ibc`+auth+bank, `protocolpool` требует `distribution` и т.д.
Несогласованный набор → ошибка вроде `module "ibc" requires "evidence" which is
disabled`.

Пример: собрать минимум без IBC:

```bash
umeshprep -enable-ibc=false -enable-feegrant=false -enable-authz=false
```

> При выключенной IBC из `capabilities` удаляется `ibc2` (это делает
> `filterCapabilities`); остальные capabilities остаются, а их набор перевалидируется
> в `patchCapabilities` — каждая должна присутствовать в результате.

## 6. Что именно делает патч app.go (AST)

Все трансформации — через `go/ast`/`go/printer` + `go/format` (никакого `sed`):

- **deprecated** (`group`, `nft`, `crisis`) удаляются всегда, если не задан
  `-enable-deprecated` (удаляются импорты, keeper-поля, регистрация в менеджере,
  store keys, `maccPerms`).
- **опциональные** удаляются по флагу: `ibc`/`transfer`/`ica`/`ibctm`,
  `upgrade`, `feegrant`, `authz`, `vesting`, `protocolpool` — аналогично.
- **`EndBlockers`** перестраиваются в `app/app.go`:
  `banktypes.ModuleName` ставится **первым** (приоритет `0`, требование v0.54);
  затем `staking→distr→mint→gov→genutil→feegrant→ibc-transfer→ibc→ica→wasm→
  protocolpool→upgrade→evidence→authz`. Неизвестные модули — в конец, в исходном
  порядке (`fixEndBlockers`).
- **Capabilities** — замена `wasmkeeper.BuiltInCapabilities()` на литерал
  `[]string{...}` из `-capabilities`; после подмены каждый запрошенный capability
  проверяется присутствием, а вызов `BuiltInCapabilities()` должен исчезнуть.
- **v0.60 upgrade handler** удаляется целиком (`removeV060Upgrade`): каталог
  `app/upgrades/v060/` удаляется, а в `app/upgrades.go` `var Upgrades` заменяется
  на пустой слайс (свежей сети он не нужен).

## 7. Частые ошибки и диагностика

| Симптом | Причина | Как поправить |
| ------- | ------- | ------------- |
| `Preflight: image not found` | Нет образа `umesh-node` | Соберите его в репозитории `Node_Umesh`: `docker build -t umesh-node:latest -f Dockerfile .` |
| `module "..." requires "..." which is disabled` | Несогласованный набор флагов `-enable-*` | Включите зависимую — например, для `ibc` нужен `evidence` (core, включён) |
| `wasmkeeper.BuiltInCapabilities() not found in app/app.go` | Версия wasmd отличается от `v0.70.3` | Задайте `-wasmd-version` под ваш дерево, либо поправьте `patchCapabilities` |
| `capability patch produced incomplete result (missing ...)` | capabilities пересекаются с удалённым IBC | Проверьте, чтобы включённые модули покрывали запрошенные capabilities (особенно `ibc2`) |
| `go build gate failed` | CGO выключен / нет C-компилятора для wasmvm | `CGO_ENABLED=1 go build ./...` вручную в `OUTPUT_DIR`; для быстрой проверки `-skip-build` |
| `git clone wasmd <ver>: exit status...` | Тег `<ver>` не найден в upstream | Убедитесь, что `-wasmd-version` существует в `CosmWasm/wasmd`, либо `-wasmd-repo` поправьте на форк |

## 8. Разработка и тесты

Тесты покрывают `parseConfig`: значения по умолчаниюю, приоритет флага над env,
разбор CSV-`capabilities`, отклонение позиционных и неизвестных флагов:

```bash
go test ./...
go vet ./...
go build -o umeshprep .
```

> `umeshprep` собирает дерево Umesh, а **сам** репозиторий `umesh-prep` хранит
> исходники утилиты. Docker-образ `umesh-node` и `docker-compose.yml живут в
> репозитории `Node_Umesh` и собираются отдельно.
