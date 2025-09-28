# Actionable Recommendations for AISH Enhancement

## 1. Guiding Principles

The following recommendations are guided by three core principles:

1.  **Increase Robustness:** Harden fragile integrations and improve testability to ensure reliability.
2.  **Improve Maintainability:** Refactor the codebase to enable comprehensive, automated testing of all components, especially the shell integration.
3.  **Reduce Complexity:** Remove speculative, unimplemented features and simplify the architecture to focus on delivering core value reliably.

## 2. High-Priority Recommendations (Immediate Action Required)

### 2.1. Refactor the Shell Hook

This remains the most critical area for improvement. The current implementation is a significant liability.

*   **Action:**
    1.  **Externalize Shell Scripts:** Move the shell script logic from Go string literals into separate `.sh` and `.ps1` files.
    2.  **Create a Dedicated Test Suite:** Implement a testing framework (e.g., `bats` for Bash/Zsh, `Pester` for PowerShell) to execute the hook scripts in a real shell environment and validate their behavior.
    3.  **Simplify Hook Logic:** Defer as much logic as possible to the Go binary, which is easier to test and maintain.

*   **Benefit:** This will make the most fragile part of the system robust, testable, and maintainable.

### 2.2. Harden the `gemini-cli` Provider

Given the constraint that this provider must remain the primary option, we must harden it against its inherent instability.

*   **Action:**
    1.  **Improve Authentication Logic:** Refactor the OAuth token acquisition from `~/.gemini` to be more robust. Add explicit checks for token expiration and provide clear, user-friendly error messages when authentication fails.
    2.  **Strengthen Response Parsing:** The current heuristic parsing is brittle. This will be improved by making the JSON parsing more defensive and enhancing the fallback mechanisms to handle unexpected API response formats gracefully.
    3.  **Isolate with Integration Tests:** Create a mock HTTP server to simulate the private API's behavior. Write a dedicated integration test suite for the `gemini-cli` provider that asserts its behavior against various success and failure scenarios (e.g., auth errors, malformed JSON, simulated API changes).
    4.  **Clarify Documentation:** Update the project's documentation to clearly state that the `gemini-cli` provider relies on an unofficial API and may be unstable. This is crucial for managing user expectations.

*   **Benefit:** While this does not eliminate the risk of the underlying API changing, it will make the provider more resilient, easier to debug when it fails, and provides a safety net of tests to quickly validate any future fixes.

## 3. Medium-Priority Recommendations (Core Architectural Improvements)

### 3.1. Refactor the Caching Architecture

*   **Action:**
    1.  **Remove `IntelligentCache`:** Delete the speculative and unimplemented [`internal/cache/intelligent_cache.go`](internal/cache/intelligent_cache.go:1).
    2.  **Consolidate on `LLMCache`:** Refactor [`internal/cache/llm_cache.go`](internal/cache/llm_cache.go:1) to be the single, authoritative caching mechanism.

*   **Benefit:** This will simplify the architecture and reduce cognitive overhead.

### 3.2. Implement Internationalization (I18n)

*   **Action:**
    1.  **Externalize Strings:** Move all user-facing strings into resource files.
    2.  **Implement a Localization Library:** Integrate a standard Go I18n library.

*   **Benefit:** This will make the application accessible to a global audience.

## 4. Low-Priority Recommendations (Code Quality and Cleanup)

*   **Refactor `main.go`:** Improve separation of concerns by moving large functions into their own files.
*   **Decouple UI from Core Logic:** Refactor core logic to return data, not call UI functions directly.
*   **Improve Error Classification:** Enhance the classifier to use more sophisticated pattern matching.

## 5. Revised Implementation Plan

1.  **Phase 1 (Stabilization):**
    *   Refactor the Shell Hook and create a robust test suite.
    *   Harden the `gemini-cli` provider and create its integration test suite.
2.  **Phase 2 (Architectural Cleanup):**
    *   Refactor the Caching Architecture.
    *   Implement Internationalization.
3.  **Phase 3 (Code Quality):**
    *   Address the low-priority code quality recommendations.