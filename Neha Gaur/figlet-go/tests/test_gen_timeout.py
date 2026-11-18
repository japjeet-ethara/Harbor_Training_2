import subprocess
import sys
import time
from pathlib import Path

# Ensure local test module and pyfiglet resolution behave like gen.py
import gen


def test_slant_width10_science_harbor_does_not_hang():
    """Regression: exact reported case must not hang generation."""
    args = ["-f", "slant", "-w", "10", "-j", "auto", "-D", "auto", "Science  Harbor"]
    start = time.monotonic()
    out, err = gen.try_render_case(args, timeout=2.0)
    elapsed = time.monotonic() - start

    # The H in slant is exactly 10 columns and cannot be placed on an empty line.
    # The generator must skip it immediately and must not record invented output.
    assert elapsed < 4.0, f"case hung or exceeded expected bound: {elapsed:.2f}s"
    assert out is None
    assert err is not None
    assert "time out" in err.lower() or "timeout" in err.lower()


def test_equal_width_glyph_can_still_render():
    """A glyph as wide as the limit is kept when empty-line smushing can place it."""
    args = ["-f", "standard", "-w", "10", "-j", "left", "-D", "left-to-right", "@"]
    assert gen.case_would_hang(args) is False
    start = time.monotonic()
    out, err = gen.try_render_case(args, timeout=2.0)
    elapsed = time.monotonic() - start
    assert elapsed < 4.0
    assert err is None
    assert out


def test_replacement_stops_after_attempt_limit(monkeypatch):
    """A skipped case must not be hunted forever, and its output must not be invented."""
    def always_fail(args, timeout=5.0):
        return None, "would time out"

    monkeypatch.setattr(gen, "try_render_case", always_fail)
    monkeypatch.setattr(gen, "case_would_hang", lambda args: False)
    gen.random.seed(7)
    failed = ["-f", "slant", "-w", "10", "-j", "auto", "-D", "auto", "Science  Harbor"]
    try:
        gen._replacement_for_failed_case(failed, set(), category="normal_word")
    except RuntimeError as exc:
        message = str(exc)
        assert "80" in message
        assert "normal_word" in message
        assert "slant" in message
    else:
        raise AssertionError("replacement search did not stop")


def test_run_pyfiglet_timeout_raises_render_timeout_for_hanging_case(monkeypatch):
    """run_pyfiglet must stop individual hanging cases via timeout."""

    def fake_run(*args, **kwargs):
        raise subprocess.TimeoutExpired(cmd=kwargs.get("args", args[0]), timeout=kwargs.get("timeout", 1))

    monkeypatch.setattr(gen.subprocess, "run", fake_run)
    try:
        gen.run_pyfiglet(["-f", "slant", "-w", "10", "Science  Harbor"], timeout=0.01)
        assert False, "expected RenderTimeoutError"
    except gen.RenderTimeoutError:
        pass
