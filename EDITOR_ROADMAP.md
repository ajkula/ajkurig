# Sprite editor roadmap

Each step is test-driven: write the failing tests listed, make them pass, keep every existing test green, then show the result before the next step.

## 1. Architecture: the editor embeds the game

- [x] 1a. The editor becomes a public library `ajkurig/spriteeditor`; `cmd/spriteedit` only calls it.
  - Tests: the existing editor tests run from `ajkurig/spriteeditor`; `go build ./cmd/spriteedit` succeeds.
- [x] 1b. The game code moves to a package; the game keeps two entry points: `main.go` (game only) and `cmd/editor` (editor with the game inside).
  - Tests: `go list -deps` of the game binary does not contain `ajkurig/spriteeditor`; the editor entry contains both the game package and `ajkurig/spriteeditor`; the game tests still pass.
- [x] 1c. Project browser: every `.spr` under the project root, in any folder.
  - Tests: sprites in sub-folders are listed; non-sprite files are not.
- [x] 1d. Closing a sprite never saves without asking: SAVE, DISCARD or CANCEL when there are changes.
  - Tests: opening another sprite with changes opens the question and keeps the file untouched; DISCARD loads the other sprite and leaves the first file as it was on disk; SAVE writes then opens; CANCEL keeps everything; history and selection are cleared after a switch.

## 2. Unbounded workspace

- [x] Parts keep their own pixel rectangle; the grid is only a background that fills the window.
  - Tests: a pixel painted outside the old canvas is kept, saved and read back; a move reaching past the old canvas is baked whole; files written before the change read back identical.

## 3. Anatomy and posing

- [x] 3a. Anatomy templates (humanoid, quadruped, fish, bird): per joint, limits on three axes for a very supple but real body.
  - Tests: a knee never bends backwards, an elbow never hyperextends, hips reach a full split, a quadruped hock bends backwards, a fish spine bends sideways only.
- [x] 3b. Rotation panel: per axis, minus and plus by 1 degree (Shift: 10), typed value.
  - Tests: each button changes only its axis by one degree; limits clamp; typed values are clamped.
- [x] 3c. Drag a hand or a foot: the elbow or knee follows within the limits; pins keep a foot or the pelvis in place; no hidden balance or twin unless switched on.
  - Tests: dragging a foot moves only that leg; a pinned foot stays on its pixel; the other leg does not move with twin OFF (the default).

## 4. Drawing by faces

- [x] Draw a part from front, back, left, right, top and bottom; the world turns to present the face; an XYZ marker shows the viewpoint.
  - Tests: painting on a face stores it on that face only; turning the world picks the matching face.

## 5. Depth per pixel, then voxels

- [x] 5a. Depth compared per pixel instead of per part.
  - Tests: an arm half behind the torso shows its front half only.
- [x] 5b. Voxel parts carved from the face drawings, rendered at any angle with the sprite palette and an outline.
  - Tests: a cube drawn on three faces carves into a cube; a render uses palette colours only.
  - [x] 5b1. Carving: a voxel stays where every drawn face shows paint; each face colours the voxels it sees first.
    - Tests: three square faces carve a full cube coloured by face; a shallow top face carves a thin slab; a part drawn from the front only stays one voxel thick.
  - [x] 5b2. Rendering: the volume turns with the part's orientation, is projected with per pixel depth and gets an outline in the darkest palette colour.
    - Tests: a cube turned to each face shows that face's colour; a quarter turn between faces shows both; the outline surrounds the silhouette.
  - [x] 5b3. Solid parts in the sheet: a part marked solid renders its volume instead of its card, saved in the file.
    - Tests: a solid cube in a pose renders like 5b2; the solid flag reads back; parts not marked keep their card render byte for byte.
  - [x] 5b4. Editor: a SOLID switch in DRAW, the preview shows the turning volume.
    - Tests: the switch marks the part and is undoable; painting a face updates the volume.

## 6. Loose ends

- [x] 6a. Closing the window asks SAVE, DISCARD or CANCEL when there are changes; in UERSSA a close during play goes back to the editor first.
  - Tests: closing a saved sprite quits at once; closing with changes asks, CANCEL keeps everything, DISCARD quits and leaves the file, SAVE writes then quits.
- [x] 6b. In UERSSA every F5 reloads the sprites from disk; a broken sprite keeps the ones already loaded.
  - Tests: an edited file shows after a reload, shared by the hero and the enemies; a missing sprite is reported and changes nothing.
- [x] 6c. The preview and the snapshot thumbnails reach paint outside the canvas.
  - Tests: paint left of the canvas shows in the preview and in a thumbnail.
