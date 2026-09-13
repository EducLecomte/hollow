# Hollow — Documentation technique

**Hollow** est un explorateur de fichiers et un éditeur de texte TUI écrit en Go (`tview`/`tcell`). Il combine la simplicité de **Nano**, l'efficacité du double panneau de **Midnight Commander** et la capacité de travailler sur des fichiers locaux **et** distants (FTP, FTPS, SFTP) ou contenus dans des archives, le tout depuis un seul terminal.

Le projet est développé avec l'aide de l'IA dans un but récréatif, pédagogique et formateur. Par convention, l'ensemble des commentaires de code et des chaînes destinées à l'utilisateur est en français.

---

## Sommaire

1. [Installation](#1-installation)
2. [Utilisation](#2-utilisation)
3. [Raccourcis clavier](#3-raccourcis-clavier)
4. [Architecture](#4-architecture)
5. [Couche VFS : les protocoles](#5-couche-vfs--les-protocoles)
6. [Mécanismes internes](#6-mécanismes-internes)
7. [Limites par protocole](#7-limites-par-protocole)
8. [Notes de sécurité](#8-notes-de-sécurité)
9. [Développement](#9-développement)
10. [Fichiers du projet](#10-fichiers-du-projet)

---

## 1. Installation

### 1.1 Binaire précompilé (Linux)

```bash
curl -sL https://raw.githubusercontent.com/EducLecomte/hollow/main/install.sh | bash
```

`install.sh` :

- détecte l'architecture (`x86_64` → `amd64`, `aarch64` → `arm64`) ;
- interroge l'API GitHub pour la dernière release (ou utilise la version passée en argument, ex. `./install.sh v1.2.0`) ;
- télécharge `hollow-linux-<arch>` et l'installe dans `/usr/local/bin/hollow` ;
- injecte une fonction shell `hollow()` dans `~/.bashrc` et/ou `~/.zshrc`.

La fonction shell enveloppe le binaire et, à chaque sortie de Hollow, fait `cd` vers le dernier répertoire local visité (voir [§6.10](#610-mémoire-du-dernier-dossier)). C'est pourquoi il faut utiliser la fonction (et non le binaire directement) pour bénéficier de ce comportement.

### 1.2 Alpine Linux

```bash
apk add --no-cache curl
curl -fsSL https://raw.githubusercontent.com/EducLecomte/hollow/main/install-alpine.sh | sh
```

`install-alpine.sh` (POSIX `sh`) fonctionne de la même manière, avec en plus la prise en charge de `doas`/`sudo` et de la variable `INSTALL_DIR` (par défaut `/usr/local/bin`).

### 1.3 Compilation depuis les sources

```bash
make build      # écrit ./hollow, injecte main.Version via -ldflags (git describe --tags)
make install    # build + copie dans /usr/local/bin
make clean
```

Sur Alpine, pour un binaire statique `musl` :

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /usr/local/bin/hollow ./cmd/hollow
```

Prérequis : Go ≥ 1.26 et un terminal avec un vrai TTY (l'interface ne fonctionne pas dans un environnement sans TTY, comme CI).

---

## 2. Utilisation

### 2.1 Ligne de commande

```bash
hollow              # ouvre le répertoire courant
hollow <chemin>     # ouvre un répertoire précis, ou édite directement un fichier
hollow -version     # affiche la version compilée
```

- Si l'argument est un **dossier**, il devient le répertoire initial des deux panneaux.
- Si l'argument est un **fichier** (existant ou non), le panneau gauche s'ouvre dans son répertoire, le fichier y est présélectionné (`initialFileSelected`), puis il est ouvert dans l'éditeur.
- En cas de panique interne, un `recover()` dans `main.go` restaure le terminal, affiche la pile et sort avec le code `1`.

### 2.2 Modes d'affichage

L'application alterne entre trois dispositions, reconstruites à la volée par `rebuildMainLayout()` :

| Mode | Disposition | Déclenchement |
| :--- | :--- | :--- |
| **Par défaut** | Explorateur (gauche, 1 part) + Visualiseur (droite, 2 parts ≈ 66 %) | État initial, retour du double panneau/déconnexion |
| **Double panneau** | Deux panneaux à 50/50 | Connexion FTP/FTPS/SFTP, ou activation du mode double panneau (`F8`/`Ctrl+T`) |
| **Éditeur** | Plein écran : numéros de ligne (4 colonnes) + zone de texte sans retour à la ligne + pied de page Nano | `Entrée` sur un fichier, ou argument fichier au lancement |

Ensemble avec ces zones :

- **Barre de chemin** (haut, fond vert) : `Path: <chemin>` du panneau actif, raccourci `~` pour le home (`utils.ShortenPath`) ;
- **Barre d'état** (bas) : rappel des raccourcis du contexte courant, ou message temporaire pendant 5 secondes ;
- **Barre latérale des favoris** (30 colonnes, à gauche) : visible avec `Ctrl+B`.

### 2.3 Navigation dans un panneau

- La liste est toujours préfixée par l'entrée `..` (index 0) ; l'élément `i` de la liste correspond à `CurrentFiles[i-1]`.
- Tri : **dossiers d'abord** (affichés en orange avec un `/` final), puis fichiers, ordre alphabétique insensible à la casse.
- La boîte d'info sous la liste affiche, pour la sélection courante : taille, date de modification, permissions, propriétaire, groupe.
- `Entrée` :
  - sur `..` : remonte d'un niveau. À la racine `/` d'un **panneau distant**, cela **déconnecte** le serveur. À la racine d'une **archive**, cela **sort de l'archive** (et ferme sa lecture).
  - sur un **dossier** : y navigue.
  - sur une **archive** (`.zip`, `.tar`, `.gz`, `.tgz`, `.tar.gz`) : l'ouvre en lecture seule via `ArchiveFS`.
  - sur un **fichier** : l'ouvre dans l'éditeur (détecteur binaire au préalable, voir [§6.2](#62-détection-des-fichiers-binaires)).

### 2.4 Connexion distante (`F3`)

Le formulaire `Connexion Réseau (FTP/SFTP)` propose : protocole (**FTP**, **FTPS**, **SFTP** — le port passe automatiquement de `21` à `22`), hôte, port, utilisateur, mot de passe masqué.

Sur succès :

- la connexion est affectée au **panneau droit** (`RightPanel`) ; le VFS et le répertoire précédents sont sauvegardés (`PreviousFS`, `PreviousDir`) ;
- l'étiquette du panneau passe à `FTP/FTPS/SFTP (hôte)` ;
- l'interface bascule automatiquement en **double panneau** et le focus va au panneau distant.

La déconnexion se fait en validant `..` depuis la racine `/` du panneau distant : la connexion est fermée, le panneau local précédent est restauré et l'affichage revient au mode par défaut avec visualiseur.

---

## 3. Raccourcis clavier

Les raccourcis ci-dessous sont implémentés dans `handlers.go` (globaux), `explorer_handlers.go` (panneaux), `viewer_handlers.go` (visualiseur), `editor_component.go` (éditeur) et `favorites.go` (favoris).

### 3.1 Globaux (tous contextes)

| Touche | Action |
| :--- | :--- |
| `F1` | Aide contextuelle (explorateur ou archive, selon le VFS du panneau actif) — **pas dans l'éditeur** (voir [§3.4](#34-éditeur)) |
| `F3` | Dialogue de connexion FTP/FTPS/SFTP — ignorée dans l'éditeur |
| `F4` | Extraire l'archive sélectionnée (raccourci global) — ignorée dans l'éditeur |
| `Ctrl + B` | Afficher / masquer la barre des favoris |
| `Ctrl + T` | Activer / désactiver le double panneau local (mode d'affichage) |
| `Ctrl + F` | Recherche globale (fuzzy finder) — **pas dans l'éditeur** (recherche dans le document, voir [§3.4](#34-éditeur)) |
| `Ctrl + C` | Ignorée (protège l'interface) |

Toute combinaison `Alt` est absorbée. Les touches `Ctrl` non listées explicitement sont filtrées par une liste blanche.

**Ordre de capture** : la capture d'entrée de l'application (`handlers.go`) s'exécute avant celle du widget focalisé (tview v0.42.0 : `Application.HandleInput` invoque d'abord `a.inputCapture`, et un retour `nil` consomme l'événement). Les raccourcis ci-dessus sont donc interceptés dans tous les contextes — **sauf dans l'éditeur** : quand la page `edit_screen` est ouverte, la capture globale laisse passer `F1` et `Ctrl+F` à la zone de texte (et ignore `F3`/`F4`), qui les gère elle-même (voir [§3.4](#34-éditeur)).

### 3.2 Explorateur (panneaux)

| Touche | Action |
| :--- | :--- |
| `Entrée` | Ouvrir un fichier / entrer dans un dossier ou une archive |
| `Tab` | Panneau opposé (double panneau) ; visualiseur (mode simple) |
| `Shift + Tab` | Favoris si visibles (depuis le panneau gauche) ; panneau opposé (double) ; visualiseur (simple) |
| `Esc` | Quitter le double panneau (si activé manuellement) |
| `F4` | Extraire l'archive (ou l'élément dans l'archive) vers le panneau opposé |
| `F5` | Créer un lien symbolique vers la cible indiquée |
| `F6` | Modifier les permissions / propriétaire / groupe (chmod/chown, option récursive) |
| `Shift + F6` (ou `F18`) | Copier **depuis** l'autre panneau (double) |
| `F7` | Créer un fichier ou un dossier |
| `F8` | Activer le double panneau local si besoin ; sinon copier la sélection vers l'autre panneau |
| `Ctrl + E` | Extraire l'archive (ou l'élément dans l'archive) vers le panneau opposé |
| `Suppr` | Supprimer l'élément sélectionné (avec confirmation) |
| `Ctrl + D` | Ajouter / retirer le **dossier sélectionné** des favoris |
| `Ctrl + K` | Préparer la copie de la sélection (mémorise chemin + VFS source) |
| `Ctrl + U` | Coller l'élément mémorisé dans le répertoire du panneau actif |
| `Ctrl + R` | Renommer le fichier ou dossier sélectionné |
| `Ctrl + X` | Quitter Hollow (avec confirmation) |

### 3.3 Visualiseur (lecture seule)

| Touche | Action |
| :--- | :--- |
| `F1` | Aide contextuelle |
| `Tab` | Favoris (si visibles), sinon panneau gauche |
| `Shift + Tab` | Panneau gauche |
| `F8` / `Ctrl + T` | Basculer en double panneau local |
| `Ctrl + X` | Quitter Hollow (avec confirmation) |
| Flèches | Défilement |

### 3.4 Éditeur

| Touche | Action |
| :--- | :--- |
| `F1` | Aide dédiée de l'éditeur (`HelpContentEditor`) |
| `Ctrl + S` | Sauvegarder |
| `Ctrl + F` | Recherche dans le document (insensible à la casse, boucle sur la fin de fichier) |
| `Ctrl + K` | Couper la ligne courante (style Nano ; répéter pour concaténer les coupures) |
| `Ctrl + U` | Coller le texte / le bloc coupé |
| `Esc` / `Ctrl + X` | Fermer l'éditeur (demande de sauvegarde s'il y a des modifications) |
| Flèches / `Page Up` / `Page Down` | Déplacement et défilement |

Le titre de l'éditeur affiche `Édition: <fichier>` et un astérisque rouge `*` préfixe le titre dès que le contenu diffère de l'original.

> **À noter** : la capture globale s'exécute avant celle de la zone de texte (voir [§3.1](#31-globaux-tous-contextes)), mais elle laisse passer `F1` et `Ctrl+F` quand la page `edit_screen` est ouverte : `F1` affiche donc l'aide dédiée de l'éditeur (`HelpContentEditor`) et `Ctrl+F` ouvre la recherche dans le document (`showSearchDialog`, insensible à la casse, boucle). `F3` et `F4` sont quant à elles ignorées dans l'éditeur.

### 3.5 Barre des favoris

| Touche | Action |
| :--- | :--- |
| `1` à `9` | Accès direct au favori n° (premier, second, …) |
| `Entrée` | Naviguer vers le favori sélectionné |
| `Suppr` | Supprimer le favori (interdit pour `Home` et `Racine`) |
| `Ctrl + N` | Renommer le favori sélectionné |
| `Tab` / `Shift + Tab` | Revenir aux panneaux |
| `Ctrl + B` / `Esc` | Masquer la barre |
| `Ctrl + X` | Quitter Hollow |

---

## 4. Architecture

```
hollow/
├── cmd/hollow/main.go        # Point d'entrée
├── internal/
│   ├── app/                  # Toute l'interface tview
│   │   ├── ui.go             # EditorApp, layout, connexion/déconnexion
│   │   ├── panel.go          # PanelState (widget d'un panneau)
│   │   ├── navigation.go     # refreshPanel, navigation, prévisualisation
│   │   ├── handlers.go       # Raccourcis globaux
│   │   ├── explorer_handlers.go  # Raccourcis des panneaux
│   │   ├── viewer_handlers.go    # Raccourcis du visualiseur
│   │   ├── editor_component.go   # Éditeur plein écran
│   │   ├── dialogs.go        # Fenêtres modales (aide, connexion, chmod, …)
│   │   ├── operations.go     # Créer, copier/coller, copier entre panneaux, extraire, supprimer
│   │   ├── fuzzy.go          # Fuzzy finder
│   │   ├── favorites.go      # Favoris persistés
│   │   └── panel_test.go     # Tests unitaires de l'état des panneaux
│   ├── vfs/                  # Abstraction de système de fichiers
│   │   ├── vfs.go            # Interface VFS + LocalFS + helpers récursifs
│   │   ├── ftpfs.go          # FTP / FTPS
│   │   ├── sftpfs.go         # SFTP (SSH)
│   │   └── archivefs.go      # .zip / .tar / .tar.gz (lecture seule)
│   └── utils/
│       ├── utils.go          # Messages d'aide, FormatSize, IsArchive, IsBinary, ShortenPath
│       └── syntax.go         # Coloration syntaxique (chroma → ANSI)
├── ressources/               # Page de présentation statique du projet + assets visuels
│   ├── index.html            # Landing page du projet
│   ├── favicon.svg           # Icône du site
│   ├── explorer.png          # Capture de l'interface
│   ├── social-preview.png    # Image Open Graph
│   └── gophers/              # Images du gopher (README + site)
├── install.sh                # Installation binaire + fonction shell
├── install-alpine.sh         # Idem pour Alpine
├── Makefile                  # build / clean / install
└── .github/workflows/release.yml  # Release automatique
```

### 4.1 Point d'entrée — `cmd/hollow/main.go`

- Drapeau `-version` ; argument positionnel optionnel = chemin initial.
- `main.Version` (valeur `"dev"`) est remplacée à la compilation par `-ldflags` (voir [§9](#9-développement)).
- Un `defer recover()` garantit la restauration du terminal et l'affichage de la pile en cas de panique.

### 4.2 L'application — `internal/app`

**`EditorApp`** (`ui.go`) est l'objet central. Ses champs notables :

- `App`, `Pages` : infrastructure `tview` ;
- `LeftPanel`, `RightPanel`, `ActivePanel` : les `*PanelState` ;
- `DualPaneMode` : état du mode double panneau local activé manuellement ;
- `PathBar`, `Viewer`, `Status`, `FavList` : zones permanentes ;
- `FilePath`, `CopiedPath`, `CopiedFS`, `Clipboard`, `LastSearch` : état éditeur et presse-papiers ;
- `Favorites`, `ShowFavs` : barre latérale ;
- `previewCancel` : annulation de la prévisualisation en cours.

`IsDualPane()` renvoie `true` si `DualPaneMode` est actif **ou** si l'un des deux panneaux est connecté à un serveur distant ; c'est lui qui pilote la reconstruction du layout.

**`PanelState`** (`panel.go`) encapsule un panneau : `ID`, `FileSystem` (le `vfs.VFS`), `CurrentDir`, `CurrentFiles`, `List` (widget), `InfoBox`, `Box` (bordure), `PreviousFS`/`PreviousDir` (mémo pour archive et déconnexion), `initialFileSelected` (pré-sélection au lancement) et `RemoteLabel` (étiquette du serveur).

**Règle d'or de la concurrence** : tout appel `tview` effectué depuis une goroutine (chargements asynchrones, prévisualisations, copies, statuts) doit passer par `e.App.QueueUpdateDraw(...)` — c'est le contrat respecté dans tout le code.

### 4.3 Helpers — `internal/utils`

- `FormatSize` : tailles lisibles (`B`, `KB`, `MB`, …) ;
- `IsArchive` : reconnaissance `.zip`, `.tar`, `.gz`, `.tgz`, `.tar.gz` ;
- `IsBinary` : détection par octet nul dans les 1024 premiers octets ;
- `GetBinaryFileDescription` : description amicale d'un fichier binaire d'après son extension (image, vidéo, exécutable, PDF, base de données, …) ;
- `ShortenPath` : remplace le répertoire home par `~` ;
- `Highlight` (`syntax.go`) : coloration syntaxique via **chroma** rendue en séquences ANSI, ensuite convertie par `tview.TranslateANSI` pour le visualiseur ;
- Les constantes `HelpMsg*` / `HelpContent*` : rappels de la barre d'état et contenus complets de l'aide contextuelle (`F1`), notamment la copie entre panneaux et le renommage (`Ctrl+R`).

---

## 5. Couche VFS : les protocoles

`internal/vfs` isole les protocoles derrière une unique interface :

```go
type VFS interface {
    List(ctx context.Context, path string) ([]FileInfo, error)
    Read(ctx context.Context, path string) (io.ReadCloser, error)
    Write(ctx context.Context, path string, data io.Reader) error
    Mkdir(ctx context.Context, path string) error
    Copy(ctx context.Context, src, dst string) error
    Remove(ctx context.Context, path string) error
    Stat(ctx context.Context, path string) (FileInfo, error)
    Chmod(ctx context.Context, path string, mode os.FileMode) error
    Chown(ctx context.Context, path, owner, group string) error
    Rename(ctx context.Context, src, dst string) error
    Symlink(ctx context.Context, target, linkPath string) error
    Close() error
}
```

`FileInfo` uniformise les métadonnées : `Name`, `IsDir`, `Size`, `ModTime`, `Permissions`, `Owner`, `Group`, `Mode`.

Trois helpers récursifs opèrent sur n'importe quel `VFS` (et entre VFS différents) :

- `CopyRecursiveBetweenVFS(ctx, srcFS, dstFS, src, dst)` : copie récursive entre deux systèmes (c'est lui qui réalise les copies entre panneaux et l'extraction) ;
- `ChmodRecursive` / `ChownRecursive` : application récursive des propriétés.

### 5.1 `LocalFS`

- `List`/`Stat` : résolution Unix du propriétaire et du groupe via `syscall.Stat_t` + `user.LookupId`/`LookupGroupId` (valeur `unknown` en cas d'échec) ;
- `Mkdir` : permissions `0755` ;
- `Copy` : récursive, avec garde-fou qui refuse de copier un répertoire dans lui-même (comparaison des chemins absolus) ;
- `Remove` : `os.RemoveAll` ;
- `Chown` : conversion des noms en UID/GID (chaîne vide = on ne touche pas à ce champ).

### 5.2 `FtpFS` (FTP / FTPS)

Basé sur `github.com/jlaffaye/ftp` :

- timeout de connexion de 5 secondes ;
- FTPS : `tls.Config{InsecureSkipVerify: true}` (certificats auto-signés acceptés) ;
- **reconnexion automatique** : chaque opération passe d'abord par `ensureConn`, qui teste la session avec une commande `NoOp` et, en cas d'échec, redial + re-login, avec notifications à l'UI via le callback `OnStatus` (`Reconnexion FTP en cours...`, `FTP reconnecté`) ;
- `Read`/`Write` : lecteurs enveloppés (`cancelableReader`, `cancelableReadCloser`) pour respecter l'annulation du contexte pendant les copies ;
- métadonnées **approximatives** : permissions affichées `rwxr-xr-x`, propriétaire/groupe `ftp`, mode `0755` ;
- `Stat` : réalisé par `List` du dossier parent puis recherche du nom ;
- `Remove` : `Delete`, avec repli sur `RemoveDirRecur` pour les dossiers ;
- `Copy`, `Chmod`, `Chown` : **non supportés** (erreurs descriptives).

### 5.3 `SftpFS`

Basé sur `golang.org/x/crypto/ssh` + `github.com/pkg/sftp` :

- authentification par mot de passe, `ssh.InsecureIgnoreHostKey()`, timeout 5 s ;
- comme `FtpFS`, `ensureConn` vérifie la connexion et relance SSH + SFTP automatiquement, avec notification `OnStatus` ;
- `Mkdir` : `MkdirAll` (crée les parents manquants) ;
- `Remove` : suppression récursive manuelle des dossiers (`removeRecursive`) ;
- propriétaire/groupe : **UID/GID numériques** exposés par SFTP (valeur `sftp` en l'absence d'info) ;
- `Chmod` : supporté nativement ;
- `Chown` : exige des **nombres** (UID/GID) — une valeur non numérique produit une erreur explicite ;
- `Copy` : non supporté (utiliser `CopyRecursiveBetweenVFS`) ;
- `Close` : ferme d'abord la session SFTP, puis SSH.

### 5.4 `ArchiveFS` (lecture seule)

- `NewArchiveFS` analyse le fichier d'après son extension : `.zip` (`archive/zip`) ou `.tar`/`.gz`/`.tgz` (`archive/tar` + `compress/gzip`) ;
- l'arborescence complète est construite **en mémoire** (nœuds `ArchiveNode` avec enfants en `map`) ;
- toutes les opérations mutatives (`Write`, `Mkdir`, `Copy`, `Rename`, `Symlink`, `Remove`, `Chmod`, `Chown`) retournent une erreur : les archives sont montées en **lecture seule** ;
- naviguer dans une archive sauvegarde le VFS hôte dans `PreviousFS`/`PreviousDir` du panneau, et `Close` de l'archive est appelé à la sortie.

---

## 6. Mécanismes internes

### 6.1 Chargements asynchrones et annulation

Toute opération longue (listage d'un panneau, connexion, ouverture de fichier, copie, extraction, ouverture d'archive) s'exécute dans une goroutine avec un `context.Context` annulable, et affiche une modale d'attente avec un bouton **Annuler** (`showLoadingDialog`). Seuls les résultats sont rendus à l'UI via `QueueUpdateDraw`. Les listes de panneaux ne sont jamais bloquantes : le chargement se fait en arrière-plan puis remplace le contenu.

### 6.2 Détection des fichiers binaires

- Dans le **visualiseur** : échantillon des 10 000 premiers octets ; si `IsBinary` (octet nul) est positif, l'aperçu est remplacé par un avertissement `[ Fichier identifié comme <description> - Aperçu désactivé ]` (en rouge), la description venant de `GetBinaryFileDescription`.
- Dans l'**éditeur** : lecture en blocs de 32 Ko ; si du binaire est détecté, une boîte de confirmation s'affiche avant d'ouvrir (l'option `force` de `openFile` permet de forcer après confirmation).

### 6.3 Prévisualisation en direct

En mode par défaut (explorateur + visualiseur), chaque changement de sélection dans le **panneau gauche** déclenche un aperçu asynchrone avec un anti-rebond de 100 ms (l'aperçu précédent est annulé) :

- **fichier** : 10 Ko lus, coloration chroma en ANSI ;
- **dossier** : arborescence textuelle (`├──` / `└──`, dossiers en orange) ;
- **binaire** : avertissement ;
- **`..`** : visualiseur vidé.

### 6.4 Double panneau et copie entre panneaux

Il faut distinguer deux concepts différents :

- le **double panneau** est un **mode d'affichage** ; il active deux colonnes synchronisées pour comparer ou manipuler deux répertoires côte à côte ;
- la **copie entre panneaux** est une **opération de fichier** ; elle duplique un élément d'un panneau vers l'autre.

`toggleDualPaneMode()` (`F8` en mode simple, ou `Ctrl+T` partout) active un **double panneau local** : le panneau droit démarre sur le même dossier que le gauche. `Esc` (dans un panneau) le désactive et restaure le visualiseur. Cela ne copie aucun fichier ; cela change seulement la disposition de l'interface.

### 6.5 Copie entre panneaux (`F8` / `Shift+F6`)

La copie est une action distincte de la disposition de l'interface :

1. La source est le panneau actif, la destination le panneau opposé (`Shift+F6` inverse le sens) ;
2. si source et destination sont identiques, l'opération est refusée ;
3. un `Stat` de la destination avec timeout de 2 s détermine s'il existe un doublon → dialogue **Écraser / Annuler** ;
4. `executeCopyBetweenPanels` lance `CopyRecursiveBetweenVFS` en arrière-plan, avec modale d'attente annulable affichant source et destination ;
5. à l'issue : les deux panneaux sont rafraîchis, et la barre d'état signale succès, annulation ou erreur.

En résumé, **le double panneau sert à voir et manipuler deux emplacements**, tandis que **la copie entre panneaux duplique un élément vers l'autre emplacement**. La source n'est pas supprimée.

### 6.6 Extraction d'archives (`F4` / `Ctrl+E`)

Deux cas :

- **depuis l'intérieur d'une archive** : l'élément sélectionné est extrait vers le dossier courant du panneau opposé ;
- **depuis un explorateur** : l'archive sélectionnée est extraite **entière** vers le dossier `<nom>_extracted` (extension retirée, y compris `.tar.gz`/`.tgz`) dans le panneau opposé. Une `ArchiveFS` temporaire est créée pour l'opération puis fermée.

Les deux cas passent par `CopyRecursiveBetweenVFS` avec modale d'annulation.

### 6.7 Copier / coller, renommer et suppression

Hollow distingue clairement la **copie**, le **renommage** et la **suppression** :

- `Ctrl+K` mémorise le chemin **et le VFS source** de la sélection ;
- `Ctrl+U` colle dans le répertoire du panneau actif : en cas de nom existant, un suffixe `_copy`, `_copy2`, … est généré ; si le VFS source diffère du VFS cible, la copie croisée (`CopyRecursiveBetweenVFS`) est utilisée, sinon la copie native du VFS ;
- `F8` / `Shift+F6` dans un double panneau lance une **copie** entre deux panneaux. L'élément est dupliqué dans le dossier de destination et reste présent dans le dossier source ;
- `Ctrl+R` renomme l'élément sélectionné dans le panneau actif via `VFS.Rename` ; les noms vides, `.`/`..` et contenant un séparateur sont refusés ;
- `F5` crée un lien symbolique via `VFS.Symlink` ; la cible est initialisée avec l'élément sélectionné et le nom du lien avec `<nom>.link` ;
- `Suppr` : confirmation, puis `Remove` et rafraîchissement du panneau.

Le presse-papiers (`Ctrl+K` / `Ctrl+U`) est donc un mécanisme de **copie préparée puis collée**, tandis que `F8` correspond à une **copie directe entre panneaux**.

### 6.8 Recherche globale (fuzzy finder, `Ctrl+F`)

- scan **synchronisé sur le VFS du panneau actif** (donc aussi sur un serveur distant) à partir de `/`, avec exclusion des dossiers système (`proc`, `sys`, `dev`, `run`, `snap`, `boot`, `tmp`, `node_modules`, `vendor`, `lost+found`, `.git`) et une borne de 10 000 fichiers collectés ;
- filtre multi-tokens en sous-chaîne, insensible à la casse, affichage limité à 100 résultats ;
- le chemin affiché est raccourci (`~`) ; `Entrée` (ou double sélection) ouvre le fichier dans l'éditeur ; `Esc` ferme (depuis la liste, le retour à la zone de saisie est conservé).

### 6.9 Favoris

- persistés dans `os.UserConfigDir()/hollow/favorites.json` (JSON indenté, `0644`) ;
- les deux premiers entrées sont **forcées** : `Home` (index 0) et `Racine` (index 1), jamais supprimables ni renommables ;
- `Ctrl+D` sur un dossier sélectionné ajoute ou retire le favori (toggle) ; seuls les dossiers sont acceptés ;
- la barre latérale affiche les 9 premiers favoris avec leur raccourci numérique ;
- sélectionner un favori navigue le panneau actif vers son chemin (et sort d'une archive en cours le cas échéant).

### 6.10 Mémoire du dernier dossier

À la confirmation de sortie, `saveLastDir()` écrit le répertoire courant du panneau local actif dans `/tmp/hollow_cwd_$USER` (si le panneau actif est distant, c'est celui du panneau gauche qui est retenu). La fonction shell `hollow()` installée par `install.sh` lit et supprime ce fichier après chaque sortie, ce qui synchronise le `cd` du shell. **Ce contrat doit être préservé.**

---

## 7. Limites par protocole

| Opération | Local | FTP/FTPS | SFTP | Archive |
| :--- | :-: | :-: | :-: | :-: |
| Listage / lecture / écriture | ✅ | ✅ | ✅ | ✅ (lecture seule) |
| Création de dossier | ✅ (`0755`) | ✅ (un niveau) | ✅ (`MkdirAll`) | ❌ |
| Suppression | ✅ récursive | ✅ (repli `RemoveDirRecur`) | ✅ récursive | ❌ |
| Copie native | ✅ (anti-auto-referentiel) | ❌ | ❌ | ❌ |
| Renommage | ✅ | ✅ | ✅ | ❌ |
| Lien symbolique | ✅ | ❌ | ✅ | ❌ |
| Chmod | ✅ | ❌ | ✅ | ❌ |
| Chown | ✅ (noms) | ❌ | ✅ (UID/GID numériques uniquement) | ❌ |
| Métadonnées fiables | ✅ (uid/gid résolus) | ⚠️ (0755, `ftp`) | ✅ (UID/GID bruts) | ⚠️ (issues de l'archive) |
| Reconnexion auto | n/a | ✅ (`NoOp`) | ✅ | n/a |

Pour les copies entre panneaux et l'extraction, `CopyRecursiveBetweenVFS` contourne l'absence de copie native : tout VFS peut être source ou destination.

---

## 8. Notes de sécurité

- **FTPS** : `InsecureSkipVerify` est forcé — les certificats serveur ne sont **pas** vérifiés (acceptables auto-signés). La confidentialité du transit est assurée, pas l'authentification du serveur.
- **SFTP** : `ssh.InsecureIgnoreHostKey()` — la clé du serveur n'est pas vérifiée.
- Les mots de passe restent en mémoire pour permettre la **reconnexion automatique** ; ils ne sont jamais écrits sur disque.
- L'authentification est **mot de passe uniquement** (aucune gestion de clés SSH).
- Les chemins saisis dans les dialogues ne sont pas échappés côté VFS : ne pas exécuter Hollow contre des serveurs non fiables.

---

## 9. Développement

```bash
make build        # ./hollow (Version injectée depuis git describe --tags, "dev" sinon)
make clean
go test ./...     # tests unitaires (rapides, sans réseau ni TTY — jamais de App.Run())
go vet ./...      # propre
```

Test unitaire unique :

```bash
go test ./internal/app/ -run TestPanelStateBasics
```

### 9.1 Releases

- pousser une tag `v*` déclenche `.github/workflows/release.yml` ;
- compilation Linux `amd64` et `arm64` avec `-ldflags="-X main.Version=$VERSION -s -w"` (la version est injectée depuis la tag) ;
- artefacts publiés en GitHub Release sous les noms `hollow-linux-amd64` et `hollow-linux-arm64` ;
- `install.sh` / `install-alpine.sh` récupèrent automatiquement la dernière release via l'API GitHub.

### 9.2 Conventions

- commentaires et chaînes utilisateur **en français** ;
- tout appel `tview` hors du thread UI passe par `App.QueueUpdateDraw` ;
- index 0 d'une liste de panneau = `..` (mappage `i` → `CurrentFiles[i-1]`) ;
- les binaires `hollow` et `bin/hollow` sont commités (pas de `.gitignore`) : ne pas committer un build local ;
- l'interface exige un TTY : vérifier par `go build`, `go vet` et les tests unitaires plutôt qu'en lançant l'app.

---

## 10. Fichiers du projet

| Fichier / dossier | Rôle |
| :--- | :--- |
| `cmd/hollow/main.go` | Point d'entrée, version, récupération de panique |
| `internal/app/` | Interface complète (12 fichiers, voir [§4](#4-architecture)) |
| `internal/vfs/` | Interface VFS et 4 implémentations + helpers |
| `internal/utils/` | Utilitaires (aide, tailles, binaire, archives, syntaxe) |
| `ressources/index.html` | Page de présentation statique du projet |
| `ressources/gophers/` | Illustrations du gopher (README + site) |
| `ressources/favicon.svg` | Icône du site |
| `ressources/explorer.png` | Capture de l'interface Hollow |
| `ressources/social-preview.png` | Image Open Graph |
| `install.sh` | Installation binaire + fonction shell de synchronisation de dossier |
| `install-alpine.sh` | Variante Alpine (`sh`, `doas`/`sudo`, `INSTALL_DIR`) |
| `Makefile` | `build`, `clean`, `install` |
| `.github/workflows/release.yml` | CI de release (tags `v*`) |
