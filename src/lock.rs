use serde::Deserialize;
use std::collections::BTreeMap;

#[derive(Debug, Default, Deserialize)]
pub struct FlakeLock {
    #[serde(default)]
    pub nodes: BTreeMap<String, Node>,
}

#[derive(Debug, Default, Deserialize)]
pub struct Node {
    pub locked: Option<Locked>,
    pub original: Option<Original>,
    pub inputs: Option<BTreeMap<String, InputRef>>,
}

/// How the input was requested. `ref_` carries the branch, tag, or (for "evergreen" inputs) a pinned commit.
#[derive(Debug, Default, Deserialize)]
pub struct Original {
    #[serde(rename = "type", default)]
    pub kind: String,
    #[serde(default)]
    pub repo: String,
    #[serde(rename = "ref", default)]
    pub ref_: String,
}

#[derive(Debug, Default, Clone, Deserialize)]
pub struct Locked {
    #[serde(rename = "type", default)]
    pub kind: String,
    #[serde(default)]
    pub owner: String,
    #[serde(default)]
    pub repo: String,
    #[serde(default)]
    pub rev: String,
    #[serde(rename = "narHash", default)]
    pub nar_hash: String,
    #[serde(default)]
    pub host: String,
    #[serde(default)]
    pub url: String,
    #[serde(default)]
    pub path: String,
}

/// A node's input target: one node name, or a `follows` path of names.
#[derive(Debug, Clone, Deserialize)]
#[serde(untagged)]
pub enum InputRef {
    One(String),
    Follows(Vec<String>),
}
