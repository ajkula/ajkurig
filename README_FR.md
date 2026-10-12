# ajkurig, en bref

Le rig d'Ajkula : un framework Go + Ebitengine pour jeux d'arcade, extrait de Hordefall, sans ses personnages ni ses musiques. Tout se règle par des valeurs, des tables à remplir ou des écrans à remplacer. La doc complète est dans [README.md](README.md), les leçons dans [LESSONS.md](LESSONS.md).

```
go run ./examples/discs        jouer à l'exemple
go test ./...                  tous les tests (sans fenêtre)
go run ./cmd/tracker           l'éditeur de musique
go run ./cmd/spriteedit hero.spr   l'éditeur de sprites
go run ./cmd/spritetool          composer, tourner et vérifier des sprites en ligne de commande
go run ./cmd/release -config examples/discs/release.json   les archives de l'exemple
```

L'exemple tient entièrement dans `examples/discs` (code, langues, `release.json`, scripts de build) : supprimer ce dossier suffit, le framework compile et ses tests passent toujours.

## Ajouter une mécanique

Une mécanique = un module : une struct avec `Name()` et seulement les méthodes dont elle a besoin. On la monte, c'est tout.

| Méthode | Quand |
|---|---|
| `Update(frame)` | simulation, 60 fois par seconde |
| `Draw(frame, screen)` | dessin du monde (dans `app.Batches` ou direct) |
| `DrawLate(frame, screen)` | après les batches, avant le bloom (étalonnage) |
| `DrawHUD(frame, screen)` | après le bloom (barres, textes, bulles) |
| `Reset(frame)` | nouvelle partie ou nouveau clip de démo |
| `Report(report)` | lignes du panneau de bench |

```go
app.World.Mount(&Bouclier{w: monde})
```

Les modules tournent dans l'ordre de montage ; `Enable`, `Unmount` et `Find` les gèrent. Chaque module est chronométré (une frame sur huit, donc gratuit) : F1 affiche le tableau, le menu principal montre le bench en direct (Select le cache). Une mécanique éprouvée dans un jeu remonte dans le framework comme module.

## Ce qu'il y a dedans

- **Musique** : tracker 16 canaux, couches adaptatives liées à des signaux nommés librement (`layer 3 action 0.15`), tempo qui monte avec `intensity`, compositeur procédural, arrangeur, éditeur (`cmd/tracker -game chemin.trk`).
- **Sons** : banque de sons synthétisés, ids, intervalle minimum ; ceux du framework (menus, voix, intro) sont remplaçables.
- **Monde** : sol en matériaux qui réagissent (l'herbe brûle, laisse de la cendre, repousse ; l'eau gèle ; la propagation augmente en canicule), statuts et réactions, météo.
- **Animation procédurale** : servo, ressorts, IK à deux os, démarche de pattes.
- **Sprites en pixels** : fichier texte `.spr`, pièces sur un squelette, poses clés, intermédiaires calculés (rotation pixel art), images figées retouchables ; éditeur `cmd/spriteedit` : image de référence à décalquer (glisser-déposer ou `-ref`), zoom à la molette pour travailler pixel par pixel ; étapes squelette, poses (cintres épaules/hanches, clichés), dessin, animation ; OUVRIR / NOUVEAU pour changer de sprite, IMPORT pour reprendre les mouvements d’un autre sprite (reciblés sur son squelette), copie de l’image de référence dans les pièces ; le jeu charge un dossier avec `sprite.LoadLibrary`, demande `hero.punch` et le dessine sur son ancrage avec `clip.Draw` ; `PeakTick` (la clé la plus longue, où le coup porte) et `KeyStart` calent les événements du jeu sur les mouvements.
- **Rotation 3D des sprites** : chaque articulation tourne aussi autour de la colonne, chaque pièce a une profondeur et des vues (ses dessins pour les autres côtés) ; le rendu raccourcit les membres tournés vers la caméra, retourne ceux vus de dos, dessine les plus proches en dernier (de dos, avant-bras derrière les bras et membres derrière le corps ; de face, l'inverse ; de profil, la largeur décide) et montre la vue du bon côté ; le cintre des épaules choisit la vue du torse. Dans l'éditeur : TOURNER en poses et animation, CÔTÉS en dessin. Mouvements en texte (`spritetool compose hero.spr hero.moves`) : direction de chaque os, `face` pour la rotation, `plant` pour garder un pied au sol, `brace` pour garder aussi le second (genou plié vers l'avant), avec un décalage optionnel du pied pour placer chaque pas, `lead` pour pencher le corps comme un pendule au-dessus du pied d'appui ; `spritetool moves`, `turnaround` et `parts` dessinent des planches de contrôle.
- **Écrans** : intro GREG's, menu sur démo d'attraction et classement, options, graphismes, audio, playlist, commandes (clavier, souris, manette, deux sticks), langue, pause, montée de niveau, initiales, game over.
- **Progression** : courbe d'XP, cartes de bonus ; l'affichage est remplaçable.
- **Bulles de dialogue** : texte qui défile sur un fond léger, avec voix, largeur et opacité réglables.
- **Release** : zips Windows, `.app` macOS universel, Linux par Docker, archives complètes.

## Mesuré (i5-6200U)

| Quoi | Avant | Après |
|---|---|---|
| Rafraîchissement du sol, 65 536 cases, chaque frame | 1 014 µs | 53 µs |
| Tick du sol, toutes les 6 frames | 969 µs | 137 µs |
| Surcoût des chronos, 20 modules × 2 phases | 12,1 µs | 3,7 µs |
| Exemple, 10 000 disques, un tick | 28 ms | 3,3 ms |

Chaque optimisation est vérifiée par un test qui compare octet par octet avec l'ancien algorithme.
