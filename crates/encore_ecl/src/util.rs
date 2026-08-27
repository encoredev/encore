/// Returns the candidate closest to `s`, or "" if nothing is close enough to be
/// a plausible typo. Among equally close candidates, those sharing a prefix
/// with `s` are preferred (so "G" suggests "GB", not "B").
pub(crate) fn suggest(s: &str, candidates: &[String]) -> String {
    #[derive(Clone, Copy, PartialEq)]
    struct Score {
        dist: i32,
        no_prefix: bool,
    }
    // better reports whether score a is strictly better than score b.
    fn better(a: Score, b: Score) -> bool {
        if a.dist != b.dist {
            return a.dist < b.dist;
        }
        !a.no_prefix && b.no_prefix
    }

    let mut best = String::new();
    let mut best_score = Score {
        dist: 3, // only suggest within edit distance 2
        no_prefix: false,
    };
    let lower = s.to_lowercase();
    for c in candidates {
        if c == s {
            continue;
        }
        let cl = c.to_lowercase();
        let sc = Score {
            dist: edit_distance(&lower, &cl),
            no_prefix: !cl.starts_with(&lower) && !lower.starts_with(&cl),
        };
        if better(sc, best_score) || (sc == best_score && !best.is_empty() && *c < best) {
            best = c.clone();
            best_score = sc;
        }
    }
    if !best.is_empty() && best_score.dist > s.chars().count() as i32 {
        return String::new(); // suggestion would replace the whole input
    }
    best
}

pub(crate) fn edit_distance(a: &str, b: &str) -> i32 {
    let ra: Vec<char> = a.chars().collect();
    let rb: Vec<char> = b.chars().collect();
    let mut prev: Vec<i32> = (0..=rb.len() as i32).collect();
    let mut cur: Vec<i32> = vec![0; rb.len() + 1];
    for i in 1..=ra.len() {
        cur[0] = i as i32;
        for j in 1..=rb.len() {
            let cost = if ra[i - 1] == rb[j - 1] { 0 } else { 1 };
            cur[j] = (prev[j] + 1).min((cur[j - 1] + 1).min(prev[j - 1] + cost));
        }
        std::mem::swap(&mut prev, &mut cur);
    }
    prev[rb.len()]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_suggest() {
        let mut kinds: Vec<String> = crate::eval::default_schema().keys().cloned().collect();
        kinds.sort();
        assert_eq!(suggest("sevice", &kinds), "service");
        assert_eq!(suggest("buckte", &kinds), "bucket");
        assert_eq!(suggest("zzzzz", &kinds), "");

        // Prefix matches are preferred among equally distant candidates.
        let cands: Vec<String> = ["B", "GB", "Gi", "KB"]
            .iter()
            .map(|s| s.to_string())
            .collect();
        assert_eq!(suggest("G", &cands), "GB");
    }
}
