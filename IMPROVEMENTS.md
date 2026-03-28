# Bifrost — Geliştirme Yol Haritası

Bu doküman, projenin mevcut durumunu analiz ederek tespit edilen eksiklikleri ve bunların nasıl giderileceğini öncelik sırasına göre detaylıca açıklar.

---

## Öncelik Sıralaması

| # | Başlık | Etki | Zorluk | Öncelik |
|---|--------|------|--------|---------|
| 1 | OpenCode MCPConfigPath Bug | Bug fix | Düşük | Kritik |
| 2 | Demo Video / GIF | Görünürlük | Orta | Yüksek |
| 3 | Snapshot Retention Limiti | Kullanılabilirlik | Düşük | Yüksek |
| 4 | Cursor Adapter | Erişim | Orta | Yüksek |
| 5 | `bifrost update` Komutu | UX | Orta | Orta |
| 6 | Plan Lock Conflict Reporting | Güvenilirlik | Düşük | Orta |
| 7 | Snapshot Boyutu Uyarısı | UX | Düşük | Orta |
| 8 | `bifrost doctor --fix` Modu | UX | Yüksek | Düşük |
| 9 | GitHub Actions Badge | Güven | Çok düşük | Düşük |
| 10 | "It works!" Kanıtı | Güven | Düşük | Düşük |

---

## 1. OpenCode MCPConfigPath Bug (Kritik)

### Problem

`internal/adapters/claude_code.go` absolute path döndürüyor:
```go
func (a *ClaudeCode) MCPConfigPath() string {
    return filepath.Join(a.homeDir, ".claude", "mcp.json")
    // Örnek: /Users/kog/.claude/mcp.json
}
```

`internal/adapters/opencode.go` relative path döndürüyor:
```go
func (a *OpenCode) MCPConfigPath() string { return "opencode.json" }
// Çalışma dizinine bağımlı — nereye yazılacağı belirsiz
```

`internal/cli/install.go`'da `installMCPConfig(a, mcpPath)` bu path'i olduğu gibi `os.ReadFile(mcpPath)` ve `os.WriteFile(mcpPath, ...)` ile kullanıyor. Relative path gelirse binary'nin çalıştığı dizine yazılır — yani kullanıcının terminal konumuna. Hiçbir uyarı yok.

### Çözüm

`opencode.go`'yu `claude_code.go` ile tutarlı hale getir:

```go
func (a *OpenCode) MCPConfigPath() string {
    return filepath.Join(a.homeDir, ".opencode", "opencode.json")
}
```

**Uygulama adımları:**

1. `internal/adapters/opencode.go` dosyasında `MCPConfigPath()` metodunu yukarıdaki gibi değiştir.
2. `internal/adapters/opencode_test.go` dosyasında MCP path testini güncelle:
   ```go
   func TestOpenCodeMCPConfigPath(t *testing.T) {
       a := newOpenCode()
       got := a.MCPConfigPath()
       // Mutlak yol olmalı, relative değil
       if !filepath.IsAbs(got) {
           t.Errorf("MCPConfigPath() returned relative path: %s", got)
       }
       if !strings.HasSuffix(got, "opencode.json") {
           t.Errorf("MCPConfigPath() = %s; want suffix opencode.json", got)
       }
   }
   ```
3. OpenCode'un MCP config dosyasının gerçek yolunu dokümantasyonda da düzelt (README'deki MCP bölümü).

**Not:** OpenCode'un gerçekte `opencode.json`'ı nereye koyduğunu teyit et — resmi OpenCode dokümantasyonuna bak. Eğer gerçekten proje kök dizinine yazılıyorsa o zaman `MCPConfigPath()` signature'ını değiştirmek ve proje root'unu parametre olarak almak gerekir.

---

## 2. Demo Video / GIF (Yüksek Öncelik)

### Problem

GitHub'a gelen ziyaretçilerin %80'i README'yi okumadan önce "bu nasıl çalışıyor?" sorusunu görmek ister. Metin açıklaması ne kadar iyi olursa olsun, bir demo yoksa insanlar ayrılır.

### Çözüm

**Araçlar:**
- macOS: [Asciinema](https://asciinema.org/) + [agg](https://github.com/asciinema/agg) (terminal kaydı → GIF)
- Alternatif: [vhs](https://github.com/charmbracelet/vhs) (script ile terminal GIF üretir, tekrarlanabilir)

**`vhs` tercih edilmeli** çünkü bir `.tape` dosyasına yazarsın ve her sürümde aynı GIF'i tekrar üretebilirsin.

**Kurulum:**
```bash
brew install vhs
```

**Demo senaryosu** (`.github/demo.tape` dosyası olarak kaydet):

```vhs
Output demo.gif

Set FontSize 14
Set Width 900
Set Height 500
Set Theme "Dracula"

# Proje başlangıcı
Type "cd ~/my-api"
Enter
Sleep 500ms

# Handoff
Type "/handoff implementing JWT refresh token rotation — auth.ts stubbed, tests missing"
# Bu kısım slash command çıktısını simüle etmek için pre-rendered text gösterebilirsin
# ya da gerçek Claude Code oturumu ekran kaydı ile yap

# Snapshot durumu
Type "bifrost status"
Enter
Sleep 1s

# History
Type "bifrost history"
Enter
Sleep 1s

# Doctor
Type "bifrost doctor"
Enter
Sleep 1s
```

**Gerçek session kaydı için daha iyi bir yaklaşım:**
1. Gerçek bir projede `/handoff` çalıştır
2. `bifrost status` çıktısını kaydet
3. Başka bir terminalde `/handin` çıktısını kaydet
4. Bu iki terminal çıktısını yan yana göster

**README'ye eklenecek yer** (The Problem bölümünden hemen sonra):

```markdown
## Demo

![Bifrost handoff demo](https://raw.githubusercontent.com/kogungor/bifrost/dev/.github/demo.gif)
```

**Yapılacaklar listesi:**
- [ ] `vhs` kur
- [ ] `.github/demo.tape` yaz
- [ ] `bifrost status`, `bifrost doctor`, `bifrost history` komutlarını içeren 45-60 saniyelik bir GIF çek
- [ ] GIF'i `.github/` dizinine koy
- [ ] README'nin en üstüne ekle (tagline'dan hemen sonra)

---

## 3. Snapshot Retention Limiti

### Problem

Her `/handoff` çağrısında `.bifrost/history/` dizinine yeni bir dosya ekleniyor. Limit yok. Aylarca kullanıldığında yüzlerce dosya birikir. Hiçbir uyarı verilmiyor.

### Çözüm A: Otomatik prune (önerilen)

`internal/snapshot/archive.go`'daki `archiveRaw()` fonksiyonunu çağırdıktan sonra eski snapshot'ları temizleyen bir `Prune()` fonksiyonu ekle:

```go
// Prune removes oldest snapshots from history, keeping at most maxKeep.
// If maxKeep <= 0, no pruning is done.
func Prune(projectRoot string, maxKeep int) error {
    if maxKeep <= 0 {
        return nil
    }

    dir := HistoryDir(projectRoot)
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    // Filter .md files only
    var mdFiles []os.DirEntry
    for _, e := range entries {
        if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
            mdFiles = append(mdFiles, e)
        }
    }

    // Entries are sorted alphabetically; since filenames are timestamps this is chronological.
    // Oldest are at the front. Remove from the front.
    if len(mdFiles) <= maxKeep {
        return nil
    }

    toDelete := mdFiles[:len(mdFiles)-maxKeep]
    for _, e := range toDelete {
        if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
            return err
        }
    }
    return nil
}
```

`internal/snapshot/snapshot.go`'daki `Write()` fonksiyonunu güncelle:

```go
func Write(projectRoot string, snap *Snapshot) error {
    if err := Archive(projectRoot); err != nil {
        return err
    }
    // Prune to keep last 50 snapshots (configurable via BIFROST.md later)
    _ = Prune(projectRoot, 50) // best-effort, don't fail the write if prune fails

    // ... rest of write logic
}
```

**Varsayılan: 50 snapshot.** Bu makul bir değer — 50 handoff yaklaşık 2-3 ay kullanıma denk gelir.

### Çözüm B: Kullanıcı yapılandırması (gelecek)

`BIFROST.md` frontmatter'ına `max_history: 50` alanı ekle. Şu an için hardcode yeterli.

### Çözüm C: Uyarı (minimal, alternatif)

Eğer prune istemiyorsan en azından `bifrost status` ve `bifrost doctor` çıktılarına uyarı ekle:

```go
// internal/cli/doctor.go içinde
historyCount := len(history)
if historyCount > 100 {
    ui.Warning(fmt.Sprintf("History  %d archived snapshots — run 'bifrost history --prune 50' to clean up", historyCount))
} else {
    ui.Success(fmt.Sprintf("History  %d archived snapshots", historyCount))
}
```

---

## 4. Cursor Adapter

### Problem

Cursor, VS Code tabanlı AI coding araçları arasında en büyük kullanıcı tabanına sahip. Eklememek, potansiyel kullanıcı tabanının büyük kısmını dışarıda bırakmak demek.

### Çözüm

Adapter sistemi tam olarak bu genişleme için tasarlanmış. Eklenmesi için sadece bir Go dosyası ve slash command dosyaları gerekiyor.

**Adım 1:** `internal/adapters/cursor.go` dosyası oluştur:

```go
package adapters

import (
    "os"
    "path/filepath"
)

// Cursor implements the Adapter interface for Cursor.
type Cursor struct {
    homeDir string
}

func newCursor() *Cursor {
    home, err := os.UserHomeDir()
    if err != nil {
        home = os.Getenv("HOME")
    }
    return &Cursor{homeDir: home}
}

func (a *Cursor) Name() string        { return "cursor" }
func (a *Cursor) DisplayName() string { return "Cursor" }

func (a *Cursor) IsInstalled() bool {
    // Cursor stores global config under ~/.cursor
    _, err := os.Stat(filepath.Join(a.homeDir, ".cursor"))
    return err == nil
}

func (a *Cursor) CommandsDir() string {
    // Cursor'ın slash command dizinini araştır.
    // Şu an için en makul tahmin:
    return filepath.Join(a.homeDir, ".cursor", "commands")
}

func (a *Cursor) MCPConfigPath() string {
    // Cursor MCP config path'i:
    return filepath.Join(a.homeDir, ".cursor", "mcp.json")
}

func (a *Cursor) InstructionFile() string { return ".cursorrules" }
```

**Adım 2:** `internal/adapters/registry.go`'ya Cursor'ı ekle:

```go
func All() []Adapter {
    return []Adapter{
        newClaudeCode(),
        newOpenCode(),
        newCursor(), // ekle
    }
}
```

**Adım 3:** Slash command dosyaları oluştur:

`cmd/bifrost/commands/cursor/handoff.md` — Claude Code versiyonunu baz al ama Cursor'ın instruction sistem yapısına göre uyarla.

`cmd/bifrost/commands/cursor/handin.md`

`cmd/bifrost/commands/cursor/plan.md`

`cmd/bifrost/commands/cursor/review.md`

**Adım 4:** `cmd/bifrost/main.go`'daki embed direktifini güncelle.

**Adım 5:** Test yaz — `internal/adapters/cursor_test.go`.

**Önemli not:** Cursor'ın gerçekte slash command'ları nasıl ele aldığını araştır. Cursor `.md` dosyalarını mı kullanıyor yoksa farklı bir format mı? Bu bağımlılık kurulmadan adapter tamamlanmış sayılmaz.

---

## 5. `bifrost update` Komutu

### Problem

`bifrost update` CLI reference tablosunda görünüyor ama `internal/cli/` altında bu komutu implement eden bir dosya yok. Kullanıcı çalıştırırsa "unknown command" alır.

### Çözüm

İki yaklaşım var:

**A) Basit: GitHub Releases'e yönlendir**

```go
// internal/cli/update.go
var updateCmd = &cobra.Command{
    Use:   "update",
    Short: "Check for a newer version of Bifrost",
    RunE:  runUpdate,
}

func runUpdate(cmd *cobra.Command, args []string) error {
    ui.Section("Update", fmt.Sprintf("Current version: %s", version.Version))
    ui.Dim("To update via Homebrew:")
    ui.Dim("  brew upgrade bifrost")
    ui.Dim("")
    ui.Dim("To update via shell installer:")
    ui.Dim("  curl -fsSL https://raw.githubusercontent.com/kogungor/bifrost/dev/install.sh | sh")
    ui.Dim("")
    ui.Dim("Latest releases: https://github.com/kogungor/bifrost/releases")
    return nil
}
```

**B) Gelişmiş: GitHub API ile sürüm kontrolü**

```go
func runUpdate(cmd *cobra.Command, args []string) error {
    current := version.Version

    // GitHub Releases API'den son sürümü al
    resp, err := http.Get("https://api.github.com/repos/kogungor/bifrost/releases/latest")
    if err != nil {
        ui.Warning("Could not reach GitHub to check for updates.")
        ui.Dim(fmt.Sprintf("Current version: %s", current))
        return nil
    }
    defer resp.Body.Close()

    var release struct {
        TagName string `json:"tag_name"`
        HTMLURL string `json:"html_url"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
        return err
    }

    latest := strings.TrimPrefix(release.TagName, "v")

    if latest == current {
        ui.Success(fmt.Sprintf("Already up to date (%s)", current))
        return nil
    }

    ui.Warning(fmt.Sprintf("Update available: %s → %s", current, latest))
    ui.Dim(fmt.Sprintf("Release notes: %s", release.HTMLURL))
    ui.Blank()
    ui.Dim("Update via Homebrew: brew upgrade bifrost")
    ui.Dim("Or: curl -fsSL https://raw.githubusercontent.com/kogungor/bifrost/dev/install.sh | sh")
    return nil
}
```

**Dikkat:** B seçeneği tek ağ çağrısı yapan bir istisna oluyor — README'deki "No network calls after installation" ifadesiyle çelişiyor. Ya bu notu güncelle ("except for explicit update checks") ya da A seçeneğiyle kal.

**Tavsiye:** B seçeneği + README güncelleme. `update` komutu açıkça user-initiated, bu kabul edilebilir bir istisna.

---

## 6. Plan Lock Conflict Reporting

### Problem

`internal/snapshot/plan.go`'daki lock mekanizması 30 saniye sonra stale lock'ı sessizce temizliyor. Eğer iki yazma girişimi 30 saniye içinde gelirse ilk kazanır, ikinci sessizce devam eder. Hata yok, uyarı yok.

### Mevcut kod (tahmini akış)

```go
// Lock acquire
lockPath := planPath + ".lock"
err := acquireLock(lockPath)
// ... write ...
// Lock release
os.Remove(lockPath)
```

### Çözüm

Lock başarısız olduğunda kullanıcıya açık bir hata ver:

```go
func acquireLock(lockPath string) error {
    f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)
    if err != nil {
        if os.IsExist(err) {
            // Lock var — ne zaman oluşturulduğuna bak
            info, statErr := os.Stat(lockPath)
            if statErr == nil && time.Since(info.ModTime()) > 30*time.Second {
                // Stale lock — temizle ve tekrar dene
                os.Remove(lockPath)
                return acquireLock(lockPath)
            }
            // Aktif lock — açık hata döndür
            return fmt.Errorf(
                "plan is locked by another process (lock: %s)\n"+
                "If this is stale, remove it manually: rm %s",
                lockPath, lockPath,
            )
        }
        return err
    }
    f.Close()
    return nil
}
```

Bu sayede kullanıcı "plan is locked by another process" mesajı görür ve ne yapması gerektiğini bilir.

---

## 7. Snapshot Boyutu Uyarısı

### Problem

Çok ayrıntılı bir snapshot (uzun karar listeleri, çok dosya) AI context'inin önemli bir kısmını tüketir. Bunu kullanan kişi farkında olmayabilir.

### Çözüm

`/handoff` sonrasında ve `bifrost status` çıktısında snapshot boyutunu göster:

```go
// internal/cli/status.go içinde
info, err := os.Stat(snapshot.SessionPath(root))
if err == nil {
    sizeKB := info.Size() / 1024
    if sizeKB > 10 {
        ui.Warning(fmt.Sprintf("Snapshot    %d KB — large snapshot consumes significant context on /handin", sizeKB))
    } else {
        ui.Section("Snapshot", fmt.Sprintf("%d KB", sizeKB))
    }
}
```

Eşik değerleri:
- `< 5 KB` → normal, uyarı yok
- `5–10 KB` → "consider trimming decisions or environment notes"
- `> 10 KB` → açık uyarı

Bu değerler kaba tahmin — gerçek token maliyeti model ve encoding'e göre değişir ama uyarı vermek yeterli.

---

## 8. `bifrost doctor --fix` Modu

### Problem

`bifrost doctor` sorunları tespit ediyor ama düzeltmiyor. Kullanıcı "OpenCode commands not registered" mesajını görünce ne yapacağını bulmak zorunda kalıyor.

### Çözüm

`--fix` flag ekle:

```go
// internal/cli/doctor.go
var doctorFix bool

func init() {
    doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Attempt to fix detected issues automatically")
    rootCmd.AddCommand(doctorCmd)
}
```

Her kontrol bloğuna fix mantığı ekle:

```go
// Komutlar kayıtlı değilse:
if !commandsRegistered {
    ui.Warning(fmt.Sprintf("%s  commands not registered", a.DisplayName()))
    if doctorFix {
        if err := installAdapterCommands(a); err != nil {
            ui.Error("  fix failed", err.Error())
        } else {
            ui.Success("  fixed — commands registered")
        }
    } else {
        ui.Dim(fmt.Sprintf("  Run: bifrost install --adapter %s", a.Name()))
    }
}

// .bifrost/ gitignore yoksa:
if !gitignored {
    ui.Warning("Gitignore  .bifrost/ not excluded")
    if doctorFix {
        if err := project.EnsureGitignore(root); err != nil {
            ui.Error("  fix failed", err.Error())
        } else {
            ui.Success("  fixed — .bifrost/ added to .gitignore")
        }
    } else {
        ui.Dim("  Run: bifrost init")
    }
}
```

Bu öncelik sıralamasında en alt sırada çünkü mevcut deneyim zaten yeterince iyi — fix modal her koşulu kapsamak zorunda, eksik bırakılırsa kafa karıştırır.

---

## 9. GitHub Actions CI Badge

### Problem

README'de CI geçip geçmediğini gösteren badge yok. Ciddi projelerin çoğunda bu var. Yokluğu dikkat çeker.

### Çözüm

`.github/workflows/` altında bir CI workflow dosyası olduğunu varsayarak README'ye ekle:

```markdown
[![CI](https://github.com/kogungor/bifrost/actions/workflows/ci.yml/badge.svg)](https://github.com/kogungor/bifrost/actions/workflows/ci.yml)
```

Eğer CI workflow henüz yoksa `.github/workflows/ci.yml` oluştur:

```yaml
name: CI

on:
  push:
    branches: [dev, main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go test ./...
      - run: go vet ./...
```

Badge'i README'nin en üstüne, tagline'dan hemen sonra ekle:

```markdown
# Bifrost

> When tokens run dry, the bridge holds.

[![CI](https://github.com/kogungor/bifrost/actions/workflows/ci.yml/badge.svg)](...)
[![Go Report Card](https://goreportcard.com/badge/github.com/kogungor/bifrost)](https://goreportcard.com/report/github.com/kogungor/bifrost)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
```

[Go Report Card](https://goreportcard.com/report/github.com/kogungor/bifrost) badge'ini de ekle — otomatik üretiliyor, sadece link yeterli.

---

## 10. "It Works!" Kanıtı

### Problem

Projeyi ilk gören biri "bunu bizzat kullanmış mısın?" diye merak eder. Teorik olarak doğru tasarlanmış ama pratikte test edilmemiş projeler yaygın. Güven oluşturmak için somut kanıt gerekiyor.

### Çözüm

README'ye kısa bir "Real-world usage" bölümü ekle. Bu, bir blog yazısı veya detaylı hikaye olmak zorunda değil — tek bir somut örnek yeterli:

```markdown
## Real-World Example

Bifrost is developed using Bifrost. When a Claude Code session approaches its
context limit mid-feature, we hand off to a fresh session:

```
/handoff plan consensus wired, deadlock detection done — MCP update_plan tool needs review
```

The next session picks up immediately:

```
/handin
```

```
  Bifrost Briefing
  Project     bifrost
  From        claude-code
  Captured    4 minutes ago
  Commit      f4da37c
  Intent      implementing

  Task
  Wire plan consensus into bifrost_update_plan MCP tool

  Active files
  - internal/mcp/tools.go — bifrost_update_plan handler (confidence: high)
  - internal/snapshot/plan.go — WritePlan, AddReviewNote (confidence: high)

  Next step
  Add review outcome validation in tools.go lines 340–380.
  Pattern: follow bifrost_write_snapshot validation at lines 180–220.
```
```

Bu bölümü bir gerçek session'dan alınan gerçek çıktıyla doldur. Uydurma değil — gerçek bir `/handin` briefing çıktısını kopyala.

---

## Uygulama Sırası Önerisi

Bu geliştirmeleri şu sırayla uygula:

```
Hafta 1: Bug + Temel UX
├── #1 OpenCode MCPConfigPath fix (30 dakika)
├── #3 Snapshot retention (2 saat)
├── #5 bifrost update komutu (2 saat)
└── #6 Plan lock conflict reporting (1 saat)

Hafta 2: Görünürlük
├── #9 CI badge (30 dakika)
├── #7 Snapshot boyutu uyarısı (1 saat)
├── #2 Demo GIF çekimi (yarım gün)
└── #10 Real-world example (1 saat)

Hafta 3-4: Yeni özellikler
├── #4 Cursor adapter (araştırma + uygulama: 1-2 gün)
└── #8 doctor --fix (2-3 saat)
```

---

## Sonuç

Bu geliştirmelerin tamamı uygulandıktan sonra Bifrost:

- **Bug'suz**: OpenCode MCP kayıt sorunu giderilmiş
- **Güvenilir**: Lock conflict ve retention limitleri çözülmüş
- **Görünür**: Demo GIF ve CI badge ile ilk izlenim güçlü
- **Geniş kapsam**: Cursor kullanıcılarına açık
- **Self-contained**: `bifrost update` çalışıyor

GitHub'a publish etmek için Hafta 1 ve Hafta 2 yeterli. Cursor adapter ertelenebilir.
