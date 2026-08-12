use crate::frb_generated::StreamSink;
use parking_lot::Mutex;

#[derive(Debug, Clone)]
pub enum StorageEvent {
    KeyCreated { key: String },
    KeyUpdated { key: String },
    KeyDeleted { key: String },
    KeyExpired { key: String },
    StorageCleared,
    CacheInvalidated { key: String },
}

impl StorageEvent {
    fn topic(&self) -> Option<&str> {
        match self {
            StorageEvent::KeyCreated { key }
            | StorageEvent::KeyUpdated { key }
            | StorageEvent::KeyDeleted { key }
            | StorageEvent::KeyExpired { key }
            | StorageEvent::CacheInvalidated { key } => Some(key),
            StorageEvent::StorageCleared => None,
        }
    }
}

/// Matches pub/sub subscription patterns against event topics (key names).
/// Supports `*` (match everything) and a `prefix.*` glob suffix; anything
/// else is an exact match, mirroring the `store.subscribe("auth.*")` example
/// in the design docs.
fn pattern_matches(pattern: &str, topic: &str) -> bool {
    if pattern == "*" {
        return true;
    }
    if let Some(prefix) = pattern.strip_suffix(".*") {
        return topic == prefix || topic.starts_with(&format!("{prefix}."));
    }
    if let Some(prefix) = pattern.strip_suffix('*') {
        return topic.starts_with(prefix);
    }
    pattern == topic
}

struct Subscriber {
    pattern: String,
    sink: StreamSink<StorageEvent>,
}

/// Fan-out hub for storage events. `StorageCleared` (which has no key) is
/// broadcast to every subscriber regardless of pattern.
pub(crate) struct EventBus {
    subscribers: Mutex<Vec<Subscriber>>,
}

impl EventBus {
    pub fn new() -> Self {
        Self {
            subscribers: Mutex::new(Vec::new()),
        }
    }

    pub fn subscribe(&self, pattern: String, sink: StreamSink<StorageEvent>) {
        self.subscribers.lock().push(Subscriber { pattern, sink });
    }

    pub fn publish(&self, event: StorageEvent) {
        let mut subscribers = self.subscribers.lock();
        // Drop subscribers whose Dart-side stream has been closed/cancelled
        // (sink.add returning Err) so the list doesn't grow unbounded.
        subscribers.retain(|subscriber| {
            let matches = match event.topic() {
                Some(topic) => pattern_matches(&subscriber.pattern, topic),
                None => true,
            };
            if matches {
                subscriber.sink.add(event.clone()).is_ok()
            } else {
                true
            }
        });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn wildcard_matches_everything() {
        assert!(pattern_matches("*", "auth.user"));
        assert!(pattern_matches("*", "theme"));
    }

    #[test]
    fn prefix_glob_matches_namespace() {
        assert!(pattern_matches("auth.*", "auth.user"));
        assert!(pattern_matches("auth.*", "auth"));
        assert!(!pattern_matches("auth.*", "authentication"));
        assert!(!pattern_matches("auth.*", "other"));
    }

    #[test]
    fn exact_pattern_matches_only_itself() {
        assert!(pattern_matches("theme", "theme"));
        assert!(!pattern_matches("theme", "theme2"));
    }
}
