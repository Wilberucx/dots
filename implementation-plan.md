# Implementation Plan: Migrating `dashboard.py` to 'Thin UI' Architecture

## 1. Background and Context

The `src/dots/ui/dashboard.py` file has evolved into a "God Object". It currently manages:
- **TUI State**: UI-specific state (scroll positions, cursor location, active tabs).
- **Business Logic**: Data fetching (`refresh_modules`), git operations (`refresh_backups`), and variant resolution.
- **Rendering**: Directly defines how the TUI is drawn.
- **Keybindings**: Defines application behavior.

This tight coupling makes testing the application logic impossible without invoking the TUI, complicates maintenance, and violates the Single Responsibility Principle.

## 2. Proposed Architecture: 'Thin UI'

We will decouple the business logic from the UI using a **Service Layer**.

- **`DotsService` (New)**: A centralized class in `src/dots/core/` responsible for data orchestration, git operations, and business rules. It will be agnostic of the TUI.
- **`TUIState` (Refactored)**: Will only manage UI-specific concerns (scrolling, active tabs, filter state). It will depend on `DotsService` to fetch data.
- **`dashboard.py`**: Will remain the entry point for the TUI, but will delegate all data operations to `DotsService`.

## 3. Implementation Details

### Approach
1. **Define Service Interface**: Create `DotsService` to encapsulate module resolution, status aggregation, and git interactions.
2. **Incremental Migration**: Extract logic from `TUIState` to `DotsService` one method at a time, ensuring tests pass at each step.
3. **Dependency Injection**: Pass the `DotsService` instance to `TUIState` during initialization.

### Changes
- `src/dots/core/services.py`: Create this new file.
- `src/dots/ui/dashboard.py`: Refactor `TUIState` to call `DotsService`.
- `tests/`: Add unit tests for `DotsService`.

### Testing Strategy
1. **Unit Tests**: Test `DotsService` methods in isolation (mocking `subprocess` and `DotsConfig`).
2. **UI Tests**: Ensure existing E2E tests pass after the refactor to guarantee no regression in user experience.
3. **Manual Validation**: Smoke test the TUI to ensure all tabs and data refresh work as expected.

## 4. Plan by Phases

### Phase 1: Define Service Layer
- Create `src/dots/core/services.py`.
- Implement `refresh_modules()` and `refresh_backups()` inside `DotsService`.

### Phase 2: Refactor `TUIState`
- Update `TUIState.__init__` to accept `DotsService`.
- Replace calls to internal `refresh_*` methods with calls to `self.service.refresh_*`.

### Phase 3: Cleanup
- Remove redundant logic from `dashboard.py`.
- Update tests.

## 5. Acceptance Criteria

- All `DotsService` methods are fully unit tested.
- `dashboard.py` is reduced in size by at least 30-40% (logic moved to service).
- The TUI behavior remains unchanged (functionally identical).
- All project-specific linting and type-checking passed.
