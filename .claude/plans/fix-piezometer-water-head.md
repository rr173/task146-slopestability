# Fix: 实测孔压计水头未参与后续边坡分析

## 问题根因

录入孔压计读数后，后续分析（`SubmitAnalysis`、`AddReadingRecord` 的重算、以及重启后的 `reconcileOne`）仍按旧的静态水位计算。三处调用点都查询了 `LatestPiezometerReading(Tx)`，但**丢弃了返回值**（`if _, err := ...`），所以 `SolveInput.WaterTableEl` 始终取静态/请求水位，实测水头从未进入求解器。湿化后的风险因此无法体现。

现有两个失败的测试印证了该 bug：
- `TestSubmitAnalysisPrefersLatestPiezometerHead`：读数 8 + 请求水位 2 → 期望 8（实测优先于请求）
- `TestZeroPiezometerHeadOverridesStaticWater`：读数 0（实测干）+ 静态水位 12 → 期望 0（实测 0 是有效的"干"读数，不可忽略）

## 单位与语义澄清

- `model.Reading.Value` 对孔压计是**水头高程（m）**，不是孔压（kPa）。因此实测水头应替换 `SolveInput.WaterTableEl`，作为"水位来源"；`resolvePore` 已能从 `waterTableEl` 推导 u，无需改动求解器。
- `MeasuredU`（kPa）字段在当前数据模型中无来源，保持未使用（不强行套用，避免引入与单位不符的语义）。
- `ErrNotFound` = 无读数（回退）；返回非 nil 的 `*Reading` 即"有实测"，**即便 Value==0** 也代表有效读数（实测到地下水位=0）。

## 优先级规则（由测试与 `resolvePore` 意图确定）

```
实测孔压计水头(最新) > 请求运行水位(>0 才算) > 静态边坡水位
```

逐测试验证：
- 读数 8 + 请求 2 → 8 ✓（实测优先）
- 读数 0 + 请求(默认0) + 静态 12 → 0 ✓（实测 0 有效）
- 无读数 + 请求 9 → 9 ✓（`TestAnalysisRetainsRunWaterTableOverride`）
- 无读数 + 请求 0 + 静态 X → X（既有行为，smoke 全流程仍通过）

## 修复方案

### 1. 新增单一辅助函数（`internal/service/service.go`）

把水位解析逻辑集中到一处，保证监测路径与重启路径**口径一致**：

```go
// resolveWaterTable applies the water-head priority rule: the latest
// piezometer reading (a head in metres) wins over a requested run water
// table, which wins over the slope's static water table. hasReading
// distinguishes "no reading exists" (fall through) from a real reading
// whose Value is 0 (a measured dry head that must be honoured). This is the
// single place that feeds the solver's water table, so the monitoring
// recompute and the restart reconcile compute identical inputs.
func resolveWaterTable(hasReading bool, measuredHead, requested, static float64) float64 {
    if hasReading {
        return measuredHead
    }
    if requested > 0 {
        return requested
    }
    return static
}
```

### 2. `SubmitAnalysis`（`analysis_service.go`，约 123-129 行）

把丢弃的查询结果接入水位，并删除现已多余的 `errors` 用法（该函数仅此处用 `errors.Is`）：

```go
var measuredHead float64
hasReading := false
rd, err := s.store.LatestPiezometerReadingTx(ctx, tx, slopeID)
if err != nil && !errors.Is(err, store.ErrNotFound) {
    return err
}
if err == nil {
    measuredHead = rd.Value
    hasReading = true
}
waterTable := resolveWaterTable(hasReading, measuredHead, in.WaterTableEl, sl.WaterTableEl)
```

> 注意：保留 `errors` 导入以继续用 `errors.Is(err, store.ErrNotFound)` 判别"无读数"。

`gin.WaterTableEl = waterTable` 已有；`Analysis.WaterTableEl = waterTable` 也已有（第 153 行），无需再改存储。这样 `analysis.WaterTableEl` 返回实测水头，满足 `TestSubmitAnalysisPrefersLatestPiezometerHead` 的断言。

### 3. `AddReadingRecord` 重算（`monitoring_service.go`，约 147-150 行）

同样接入。此处"最新读数"含刚录入的那条（`CreateReading` 已写库，`LatestPiezometerReadingTx` 按 `ts DESC` 取最新），口径与 SubmitAnalysis 一致：

```go
var measuredHead float64
hasReading := false
rd, err := s.store.LatestPiezometerReadingTx(ctx, tx, slopeID)
if err != nil && !errors.Is(err, store.ErrNotFound) {
    return err
}
if err == nil {
    measuredHead = rd.Value
    hasReading = true
}
waterTable := resolveWaterTable(hasReading, measuredHead, last.WaterTableEl, sl.WaterTableEl)
```

> 此处请求水位取 `last.WaterTableEl`（上一条分析持久化的水位），保持"按上次分析口径重算"的现有语义，只是把实测水头叠加到优先级最前。

### 4. 重启 reconcile（`reconcile.go`，约 68-71 行）

用**非事务**版 `LatestPiezometerReading`（与该文件其余读路径一致，均为 `s.store.Xxx(ctx, ...)`），同样接入：

```go
var measuredHead float64
hasReading := false
rd, err := r.s.store.LatestPiezometerReading(ctx, slopeID)
if err != nil && err != store.ErrNotFound {
    return 0, 0, err
}
if err == nil {
    measuredHead = rd.Value
    hasReading = true
}
waterTable := resolveWaterTable(hasReading, measuredHead, last.WaterTableEl, sl.WaterTableEl)
```

这样重启后的 `current_f` 与在线监测重算用同一公式、同一水位来源 → **重启前后口径一致**（`smokeRestartRecovery` 通过）。

### 5. `SearchCritical`（`analysis_service.go`，约 260 行）

临界滑面网格搜索当前用 `sl.WaterTableEl`，且其基线（无加固）天然排除加固，但未纳入实测水头。为保持"实测水头优先参与后续分析"的一致性，同样接入：

```go
var measuredHead float64
hasReading := false
rd, err := s.store.LatestPiezometerReadingTx(ctx, tx, slopeID)
if err != nil && !errors.Is(err, store.ErrNotFound) {
    return err
}
if err == nil {
    measuredHead = rd.Value
    hasReading = true
}
waterTable := resolveWaterTable(hasReading, measuredHead, 0, sl.WaterTableEl)
gin := geotech.SolveInput{Profile: prof, Layers: layers, N: 20, WaterTableEl: waterTable}
```

> 搜索无用户请求水位（SearchCriticalInput 不含水位字段），故 requested 传 0，退回静态水位；有实测则用实测。保证临界滑面搜索同样反映湿化水位。

## 验证

1. `go test ./internal/service/ ./internal/geotech/` —— 两个先前失败的测试通过，其余既有测试不回归。
2. `go test ./internal/selfcheck/`（若 smoke 为 `Run()` 而非测试）—— 至少 `go vet ./...` 与 `go build ./...` 通过；手动核对：
   - `smokePorePressurePriority`：录入高读数后新分析 F < 干基线 F（实测水头抬升 u → F 降）。
   - `smokeRestartRecovery`：重启前后 `current_f` 不变（同一水位公式）。
   - `smokeFullFlow`/`smokeCriticalSearch` 等：水位仍为静态 0（无读数），行为不变。

## 不改动

- `resolvePore` / `Slices` / 各求解器：语义正确，实测水头经 `WaterTableEl` 进入即可，无需改 geotech。
- `MeasuredU` 字段：保留，不在本次强行套用（避免单位不符）。
- 存储 schema / handler / model：无需改动。
