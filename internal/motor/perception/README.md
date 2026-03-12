# 2D Semantic Perception — EVA's Eyes

## Overview

The perception system gives EVA continuous visual awareness through the device
camera. Video frames (JPEG) received via WebSocket are analyzed by Gemini Vision
to detect objects, identify scenes, and assess risk factors. Results are stored
as a graph structure in NietzscheDB's `eva_perceptions` collection using
Poincare ball coordinates (hyperbolic geometry).

## Architecture

```
Camera (JPEG frames via WebSocket)
    |
    v
browser_voice_handler.go (case "video")
    |
    v
PerceptionHandler.SubmitFrame()     <- async, non-blocking, buffered (5 frames)
    |
    v
PerceptionEngine.AnalyzeFrame()    <- rate-limited (1 frame / 3s default)
    |  Uses Gemini Vision (gemini-2.0-flash-exp)
    v
SceneAnalysis {objects, scene_type, lighting, activity, risk_factors}
    |
    v
PerceptionStore.StoreScene()       <- NietzscheDB graph write
    |
    +-- Scene2D node (low magnitude, near Poincare origin)
    +-- N x Object2D nodes (higher magnitude, children)
    +-- N x CONTAINS edges (Scene -> Object)
    +-- M x SPATIAL_NEAR edges (nearby objects, dist < 0.25)
    +-- 1 x TEMPORAL_NEXT edge (previous scene -> current scene)
```

## Files

| File | Purpose |
|------|---------|
| `engine.go` | Gemini Vision integration, frame analysis, rate limiting |
| `store.go` | NietzscheDB graph operations, Poincare coordinate mapping |
| `handler.go` | Async pipeline orchestration, session lifecycle |

## NietzscheDB Collection

- **Name**: `eva_perceptions`
- **Dimension**: 128 (Poincare ball)
- **Metric**: `poincare` (hyperbolic hierarchy)
- **TTL**: 30 seconds (ego-cache, nodes expire if not re-observed)

### Node Types

| Label | Description | Magnitude |
|-------|-------------|-----------|
| `Scene2D` | Full frame analysis (parent) | 0.1-0.3 (near origin) |
| `Object2D` | Individual detected object (child) | 0.3-0.6 (deeper) |

### Edge Types

| Type | From -> To | Description |
|------|-----------|-------------|
| `CONTAINS` | Scene2D -> Object2D | Scene contains object |
| `SPATIAL_NEAR` | Object2D -> Object2D | Objects close in frame (< 0.25 normalized dist) |
| `TEMPORAL_NEXT` | Scene2D -> Scene2D | Consecutive frames |
| `OBSERVED_BY` | Scene2D -> Patient | Who was observed |
| `HEBBIAN_VISUAL` | Object2D -> Concept | Cross-modal link (visual + verbal mention) |

## Gemini Tools

| Tool | Description |
|------|-------------|
| `open_camera_analysis` | Activates camera + perception pipeline |
| `get_perception_status` | Returns current scene state (objects, risks, activity) |

## Hyperbolic Coordinate Mapping

Objects are projected into 128-dim Poincare ball coordinates:

- **Scene2D**: Low magnitude (0.15), position determined by frame hash
  - dim[0-1]: angular position from hash
  - dim[2]: people count
  - dim[3]: object count
  - dim[4-10]: scene type encoding

- **Object2D**: Higher magnitude (0.3-0.6 based on confidence)
  - dim[0-1]: angular position from 2D frame location
  - dim[2-3]: bounding box size
  - dim[10-19]: category encoding
  - Inherits 10% of parent scene direction

This ensures the hyperbolic hierarchy rule: `parent.magnitude < child.magnitude`.

## Rate Limiting

- Default: 1 analysis every 3 seconds (configurable)
- Frame buffer: 5 frames (non-blocking, drops if full)
- Gemini model: `gemini-2.0-flash-exp` (fast, cheap)
- Temperature: 0.1 (deterministic scene descriptions)

## Integration Points

- **Hebbian LTP (Phase XII.5)**: When the user mentions an object that EVA sees,
  a `HEBBIAN_VISUAL` edge is created between the perception node and the semantic
  concept. "Nodes that fire together, wire together."

- **Entropy Daemon (Phase XIII)**: If an object disappears between frames, the
  AgencyEngine detects the energy drop and can generate a surprise event.

- **Ego-Cache TTL**: Perception nodes expire after 30s. Persistent objects that
  keep being re-observed get fresh nodes, creating a temporal chain. Objects
  that disappear naturally decay via the TTL.
