# ajkurig, en bref

Le rig d'Ajkula : un framework Go + Ebitengine pour jeux d'arcade, extrait de Hordefall, sans ses personnages ni ses musiques. Tout se règle par des valeurs, des tables à remplir ou des écrans à remplacer. La doc complète est dans [README.md](README.md), les leçons dans [LESSONS.md](LESSONS.md).

```
go run ./examples/discs        jouer à l'exemple
go test ./...                  tous les tests (sans fenêtre)
go run ./cmd/tracker           l'éditeur de musique
go run ./cmd/spriteedit hero.spr   l'éditeur de sprites
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
- **Sprites en pixels** : fichier texte `.spr`, pièces sur un squelette, poses clés, intermédiaires calculés (rotation pixel art), images figées retouchables ; éditeur `cmd/spriteedit`.
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
