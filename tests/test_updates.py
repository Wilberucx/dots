from dots.core.updates import is_newer

def test_version_comparison():
    assert is_newer("0.8.0", "0.7.1") is True
    assert is_newer("1.0.0", "0.9.9") is True
    assert is_newer("0.7.1", "0.7.1") is False
    assert is_newer("0.7.0", "0.7.1") is False
    assert is_newer("v0.8.0", "0.7.1") is True
    assert is_newer("0.8.0", "v0.7.1") is True

def test_invalid_versions():
    assert is_newer("abc", "0.7.1") is False
    assert is_newer("0.8.0", None) is False
