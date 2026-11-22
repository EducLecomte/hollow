# DIRECTIVES AGENT (AGENTS.md) - FLUTTER & OPENCODE

## 1. ENVIRONNEMENT & SÉCURITÉ

- **OS & Shell :** Debian Linux avec Bash (`/bin/bash`).
- **Commandes :** Utiliser la syntaxe Bash standard et les outils usuels (`grep`, `find`, `ls`, `rg`, `fd`).
- **Outils Flutter :** Utiliser `flutter analyze`, `flutter test`, et `dart format`.
- **Sécurité :** Interdiction stricte d'exécuter `sudo` sans confirmation explicite de l'utilisateur.
- **Langue :** Répondre **en français** (explications, messages, interactions). L'utilisateur communique en français ; ne passer en anglais que sur demande explicite.

## 2. RAISONNEMENT ET CONVERGENCE

Tu es un agent de développement logiciel expert en **Flutter et Dart**, travaillant sur des projets complexes.
La correction, la compréhension de l'architecture et la vérification sont prioritaires sur la rapidité.

### Raisonnement

- Pour les tâches complexes, prends le temps nécessaire pour comprendre la cause racine avant de modifier le code (ex: cycle de vie des widgets, fuites de mémoire, re-renders inutiles).
- **Évite les boucles de raisonnement improductives :**
  - Ne répète pas une hypothèse sans nouvelle information.
  - Ne réexamine pas une décision déjà validée sans raison valable.
- **Privilégie une progression claire :** Comprendre → Vérifier → Agir → Tester (`flutter test`) → Corriger → Terminer.

### Échecs et répétitions

- Après un échec (ex: erreur de compilation Dart, échec de test), analyse la cause avant de réessayer. Ne répète jamais mécaniquement une commande identique (`flutter run`, etc.).
- Après deux échecs similaires sans progrès significatif, change d'approche ou formule une nouvelle hypothèse.

### Arrêt et convergence

- Une tâche est terminée lorsque la modification demandée est réalisée, testée (`flutter test` / `flutter analyze`) et qu'aucune erreur ou avertissement bloquant ne subsiste.

## 3. ARCHITECTURE & CODE (FLUTTER / DART)

- **Respect du style existant :** Conforme-toi aux conventions Dart (Effective Dart), l'utilisation des linters configurés (`analysis_options.yaml`) et aux motifs de la codebase (Provider, Riverpod, Bloc/Cubit, GetX, etc.).
- **Bonnes pratiques UI & Widgets :**
  - **Immuabilité :** Utiliser `const` pour les constructeurs de widgets chaque fois que c'est possible afin d'optimiser les performances de rendu.
  - **Découpage :** Éviter les méthodes de construction trop longues (`build` géants) ; extraire les sous-parties en de petits `StatelessWidget` ou `StatefulWidget` dédiés.
  - **Gestion d'état :** Respecter strictement la brique de gestion d'état choisie dans le projet (pas de manipulation hasardeuse de `setState` s'il y a un gestionnaire d'état global/local en place).
- **Modifications chirurgicales :** Inspecte le contexte et diagnostique la cause racine avant d'éditer. Modifie uniquement le code nécessaire.
- **Qualité du code :**
  - Aucun bloc d'erreur vide (`catch` masqué ou `catch (_) {}` sans journalisation ou traitement interdit).
  - Gestion rigoureuse des types nuls (`null safety` stricte).

## 4. GESTION DU CONTEXTE

- Priorise la lecture ciblée (plages de lignes `start_line`-`end_line`) plutôt que d'ouvrir de volumineux fichiers entiers sans nécessité.
- Évite de relire des fichiers ou de réexécuter des commandes (`flutter pub get`, etc.) dont le résultat est déjà présent dans l'historique récent.

## 5. SUIVI ET SUIVI D'AVANCEMENT

- **Fichier `TASKS.md` :**
  - À créer et maintenir à la racine **uniquement** si une tâche complexe comporte plusieurs étapes indépendantes (`- [ ]` / `- [x]`).
- **Mémoire projet (MCP RAG) :**
  - Enregistrer dans le RAG les choix architecturaux majeurs (ex: choix du state management, design system, gestion des routes/navigation).

## 6. DOCUMENTATION PROJET

- **`DOC.md` :** À mettre à jour si l'architecture globale ou les flux de données changent.
- **`README.md` :** À mettre à jour si les commandes d'installation, de build (ex: `flutter build`, `build_runner`) ou les dépendances majeures évoluent.

## 7. REGISTRE DES ERREURS

- Consulter le registre ci-dessous avant d'exécuter des correctifs sur des modules complexes.

### Active Knowledge & Error Log

*Format d'entrée : Date/Contexte | Problème rencontré | Cause racine | Règle préventive*

- Ex: 2026-06 | `setState()` called during build | Modification de l'état asynchrone dans le constructeur ou initState | Utiliser des callbacks ou post-frame callbacks si nécessaire.
- 2026-10 | `Tried to make a child into a parent of itself` (focus_manager) + `RenderFlex overflowed` | Même `FocusNode` partagé entre un widget `Focus(onKeyEvent:)` et le `TextField` imbriqué | Ne jamais passer le même `FocusNode` à `Focus` et au `TextField` enfant ; laisser `Focus` créer son propre nœud (sans `focusNode:`) et garder le nœud sur le `TextField`.
- 2026-10 | Refonte « bibliothèque de scénarios unifiée » (commit 1d58596) rejetée par l'utilisateur | Choix de design imposé sans validation (dialogues fusionnés, bascule de mode automatique) | Conserver l'approche par mode : `ScenariosDialog(currentMode:, onScenarioSelected:)` + `TroubleshootingDialog` séparés ; valider avant toute refonte UI de ce type.
- 2026-10 | Texte de stats rendu incorrect : `1/[Instance of 'NetworkDevice'...].length équipements en ligne` (test widget, `RenderFlex overflowed`) | L'interpolation `$identifiant.prop` ne résout PAS l'accès de propriété : `$devices.length` est interprété comme `$devices` + littéral `.length` | Toujours utiliser `${devices.length}` (accolades) pour tout accès de propriété/indice en interpolation ; le `$simple` ne convient qu'à un identifiant nu.
- 2026-10 | Panneau bas : la vue d'ensemble réseau (commit 833d524) a été remplacée à la demande de l'utilisateur par un terminal interactif | Refonte UI imposée sans validation (la vue d'ensemble a remplacé le terminal que l'utilisateur voulait garder) | Conserver l'approche : `NetworkPanel` = `InteractiveTerminal` à gauche (menu d'équipement synchronisé à `SelectionCubit.selectDevice`) + logs système à droite ; le terminal miroite uniquement les logs utilisateur du moteur, jamais les logs système.
- 2026-10 | Import de topologie : `type int is not a subtype of type double in type cast` (commit bcf8dfc) | `NetworkDevice.fromJson` faisait `json['position']['dx'] as double` alors que `jsonDecode` rend un `int` pour `"100"` (sans partie décimale) | Pour tout nombre JSON susceptible d'être int ou double : `(valeur as num).toDouble()` (ou `.toInt()`), jamais `as double` direct.
- 2026-10 | `undefined_getter` (`.ip`/`.mask`) sur un record positionnel `(String, String)?` retourné par un helper (point `lxc`, commit 5aed0e1) | Un record positionnel n'expose pas de getters nommés — seul un record nommé `({String ip, String mask})?` permet `result.ip`/`result.mask` | Déclarer le type de retour en record nommé dès qu'on accède aux composants par nom ; ne pas changer un record positionnel en nommé après coup sans vérifier tous les call sites.
- 2026-10 | Backspace/Suppr dans le terminal interactif supprimait l'équipement sélectionné sur le canevas au lieu d'éditer le texte du champ | Le `CallbackShortcuts` racine de `network_simulator.dart` lie `backspace`/`delete` → `_deleteSelected` ; la propagation clavier Flutter 3.47 est feuille→racine sur la chaîne de focus, donc ce raccourci interceptait la touche avant `DefaultTextEditingShortcuts` | Dans `_handleKeyEvent` du terminal, intercepter backspace/delete quand `_focusNode.hasPrimaryFocus`, éditer le texte puis retourner `KeyEventResult.handled` pour bloquer la remontée. Pièges API (Flutter 3.47) : `KeyEvent` n'a PAS `isControlPressed` (utiliser `HardwareKeyboard.instance.isControlPressed`) ; `TextEditingController` n'a PAS `deleteText` (reconstruire le `TextEditingValue` via `replaceRange`).
- 2026-10 | Crash aléatoire pendant le ping terminal (démo entreprise autonome, ARP vide) — crash du ticker sans stack trace exploitable | Tout `firstWhere` sans `orElse` sur `devices`/`links` lève `StateError` ; dans le tick (`updateSimulation` dans un `setState` du Ticker) ou dans `CustomPainter.paint`, l'exception remonte dans le pipeline de frame (crash release opaque / écran d'erreur debug). Sites identifiés et durcis : `RoutingHandler.findL2PhysicalPath` (nœud/voisin), `NetworkEngine.sendBroadcast` (source), `DhcpHandler.startDhcpDiscover` (appareil), `LinkPainter` (liens orphelins + lien temporaire + paquets) | Toute recherche par ID sur le chemin de simulation/dessin doit être infaillible : helper local `_findDevice` → `null` + `continue`/`return` (ou sentinelle `id: 'deleted'` déjà en usage), jamais `firstWhere` nu ni `throw StateError` ; les callers de `findL2PhysicalPath` gèrent déjà `path.length < 2` (retour `[]` sûr).
- 2026-10 | Zoom molette « zoom arrière » : `getScale()` du test (et l'`InteractiveViewer` lui-même) lisait le scale à `1.0` après un zoom-out, alors que `transform[0]` valait bien le scale attendu | Dans Flutter 3.47, `Matrix4` vient de `package:vector_math/vector_math_64.dart` (plus `dart:ui`) ; `getMaxScaleOnAxis()` = `sqrt(max(|colonneX|², |colonneY|², |colonneZ|²))`. L'app écrivait `Matrix4.translationValues(tx,ty,0)..scaleByDouble(s, s, 1.0, 1.0)` : axe z laissé à 1.0 → `max(s, 1.0) = 1.0` dès que `s < 1` (zoom-out). L'app lisait `transform[0]` (x) pour ses propres calcs → incohérence invisible côté app, mais visible par `getMaxScaleOnAxis()` (tests + pinch natif, qui partait d'un scale erroné après un wheel-out) | Écrire la matrice de transformation avec l'axe z mis à l'échelle comme x/y : `..scaleByDouble(s, s, s, 1.0)` (`sw = 1.0` conserve la translation non échelonnée). Sites corrigés : `topology_canvas.dart` (zoom molette), `network_simulator.dart` (`_zoomBy`), `fit_view_coordinator.dart` (animation fit). `Matrix4.scale(s)` fait la même chose mais est déprécié dans vector_math 2.4.2.
- 2026-10 | Câbles multiples entre deux switchs invisibles (superposés) | `LinkPainter` et `_getLinkAt` (hit-test) utilisaient tous les deux le centre de chaque équipement (`position + Offset(30, 30)`) comme extrémités : tous les liens d'un même couple se tracent donc exactement par-dessus | Factoriser le calcul des extrémités dans `LinkPainter.computeLinkEndpoints` (statique) : décalage perpendiculaire symétrique selon le rang du lien dans le groupe « même couple non ordonné », trié par sens source→cible pour une stabilité indépendante de l'ordre de création ; réutiliser dans `paint()` ET dans `_getLinkAt` du canvas (sinon les câbles décalés ne sont plus sélectionnables/hoverables). Pas fixe `parallelGap = 10.0`, lien médian sur l'axe si nombre impair.
- 2026-10 | « Impossible d'augmenter le nombre de ports au-delà de 6 » dans les propriétés d'un switch | Le dropdown « Nombre de ports » (`SwitchSection`) appelait `onPortCountChanged` qui ne faisait QUE `setState(() => portCount = value)` : la variable locale changeait mais la liste `tempInterfaces` (dont la grille de ports est issue) n'était jamais régénérée | Dans `DevicePropertiesDialog`, `onPortCountChanged` doit réaligner `tempInterfaces` sur le nouveau compte : helper `_syncSwitchPorts(count)` qui conserve `vlan1` + les autres interfaces, réutilise les `Port N` existants (préservation de la config PVID/trunk) et ajoute/supprime les manquants. Le canvas ne dessine PAS les ports (un câble = simple segment centre→centre via `computeLinkEndpoints`), donc interconnecter > 6 câbles n'entraîne ni overflow ni clipping ; seule contrainte : le nom d'interface doit exister des deux côtés (le hit-test et le statut « connecté » sont par appareil + nom d'interface).
- 2026-10 | Câbles multiples qui « sortent du cadre » et semblent raccordés à rien (décalage perpendiculaire des extrémités) | `computeLinkEndpoints` déplaçait `p1`/`p2` (les extrémités) perpendiculairement : les câbles d'un faisceau ne partaient plus du centre des boîtiers, donc au-delà de ~6 ils paraissaient flotter | Remplacer les lignes droites par des **courbes de Bézier quadratiques** ancrées au centre des deux équipements : `computeLinkEndpoints` retourne maintenant `(p1, ctrl, p2)` où `p1`/`p2` sont les centres fixes et `ctrl` (point de contrôle) est décalé perpendiculairement → arcs concentriques concentriques, le lien médian reste droit. Dessin via `Path.quadraticBezierTo` ; le hit-test `_getLinkAt` échantillonne la courbe (`LinkPainter.sampleBezier`, 24 segments) en gardant la règle « câble le plus proche gagne ». `arcGap = 8.0`. Toute modification de la géométrie doit rester en phase entre `paint()` et `_getLinkAt` (mêmes helpers).
- 2026-10 | Migration helpers `scenarioDefinition`/`demoStep` : contenus FABRIQUÉS (labels, actions, cibles, topologies inventés) + overwrite de `containers_bridge_scenario.dart` dont l'arbre de travail avait divergé de HEAD | L'agent a réécrit les fichiers à partir de sa mémoire du « présumé » contenu au lieu de le relire ; `git checkout HEAD --` a alors rétabli l'original mais l'écriture intermédiaire fabriquée a failli s'y substituer | AVANT toute migration/réécriture : `git diff` + `git show HEAD:<fichier>` pour récupérer le contenu réel de référence ; migrer en conservant TOUTES les clés (spread `{...demoStep(...), 'targetIp': ...}` pour les clés non couvertes par `demoStep`, ex: `targetIp` dans containers_bridge/routing) ; APRÈS écriture, vérifier `diff` des sections `String get` (topologie) contre HEAD = identique, puis `dart format` + `flutter analyze`.
- 2026-10 | STP (802.1D) : implémentation initiale par « chaque non-racine garde son lien le moins coûteux » ne cassait PAS la boucle (la racine gardait tous ses ports en forwarding → l'inondation passait toujours sur les 2 liens inter-switch, PC2 recevait 2 copies) | La règle « meilleure liaison locale » produit un graphe qui, vu du côté racine, conserve encore tous les liens redondants : la racine n'a pas de lien « à bloquer », donc la boucle subsiste côté racine | Utiliser une **MST de Kruskal** (union-find) sur le domaine des switches STP : trier les liens internes au domaine par (coût de liaison, ID de lien) croissant, ne garder que les liens qui ne créent pas de cycle (union), mettre en `blocking` les liens créant une boucle. Résultat déterministe : exactement un lien reste actif entre deux équipements redondants. Le STP est recalculé à chaque tick (`_stpHandler.update()` en tête de `updateSimulation`), jamais sérialisé (état runtime) ; `stpEnabled` est un champ de `NetworkDevice` (par défaut `false`) sérialisé dans `toJson`/`fromJson`. Le port bloqué doit être filtré à DEUX endroits : `floodPacket` (inondation) ET `findL2PhysicalPath` (routage L2, egress + ingress). Ports vers un équipement hors domaine (PC, switch sans STP) = toujours forwarding.
- 2026-10 | L'agent a répondu en anglais sur un projet dont l'utilisateur communique en français | Aucune consigne de langue dans le contexte ; l'agent a basculé en anglais à un moment | Toujours répondre en français (règle ajoutée en section « Environnement ») ; ne changer de langue que sur demande explicite.
- 2026-10 | Tests catalogue : 2 tickets cyber (firewall_response + capacity_response) produisaient la même `data` (5 uniques au lieu de 6) | Les mutations `withFirewall(victim, enabled: false, rules: [])` et `withLinkType(links, 'link1', copper100)` étaient des **no-ops** : les valeurs ciblées étaient déjà les défauts de `NetworkDevice`/`NetworkLink`. Les JSON manuscrits d'origine ne contenaient pas toutes les clés que `toJson()` émet, donc les deux littéraux différaient ; après migration vers `topologyJson()`, les no-ops produisent des strings identiques | Une mutation de topologie doit **changer** l'état initial par rapport aux défauts du modèle ; sinon elle est invisible dans la sérialisation et deux scénarios partagent la même `data`. Vérifier que chaque mutateur appliqué dans un scénario produit un diff réel (comparer le JSON avant/après).

## 8. CONVENTIONS GIT

- Commits atomiques : un changement logique par commit.
- Conventional Commits : `<type>(<scope>) : <description>` (`feat`, `fix`, `refactor`, `docs`, `style`, `test`, `chore`).
- Ne créer de commit qu'à la demande explicite de l'utilisateur ou en fin de tâche validée.
