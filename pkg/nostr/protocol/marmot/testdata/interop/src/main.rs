// mls-interop: Interop test binary for Go↔Rust MLS (NIP-EE) validation.
//
// Reads JSON commands from stdin (one per line), processes with MDK,
// writes JSON results to stdout. State persists across commands.
//
// Commands:
//   generate_key_package  → outputs kind 443 event JSON
//   process_key_package   → validates a Go-generated kind 443 event
//   create_group          → creates group from member key packages, outputs welcome + message
//   process_welcome       → joins group from kind 444 welcome rumor
//   create_message        → encrypts a message, outputs kind 445 event
//   process_message       → decrypts a kind 445 event

use base64::engine::general_purpose::STANDARD as BASE64;
use base64::Engine;
use mdk_core::prelude::*;
use mdk_core::messages::MessageProcessingResult;
use mdk_core::MDK;
use mdk_memory_storage::MdkMemoryStorage;
use nostr::event::builder::EventBuilder;
use nostr::{Event, EventId, JsonUtil, Keys, Kind, RelayUrl, UnsignedEvent};
use openmls::prelude::tls_codec::Deserialize as TlsDeserialize;
use openmls::key_packages::KeyPackageIn;
use serde::{Deserialize, Serialize};
use std::io::{self, BufRead, Write};

#[derive(Deserialize)]
#[serde(tag = "cmd")]
enum Command {
    #[serde(rename = "generate_key_package")]
    GenerateKeyPackage {
        relay: String,
    },
    #[serde(rename = "process_key_package")]
    ProcessKeyPackage {
        event_json: String,
    },
    #[serde(rename = "create_group")]
    CreateGroup {
        member_kp_event_json: String,
        name: String,
        relay: String,
    },
    #[serde(rename = "process_welcome")]
    ProcessWelcome {
        rumor_json: String,
    },
    #[serde(rename = "create_message")]
    CreateMessage {
        mls_group_id_hex: String,
        content: String,
    },
    #[serde(rename = "process_message")]
    ProcessMessage {
        event_json: String,
    },
    #[serde(rename = "get_group_info")]
    GetGroupInfo {},
    #[serde(rename = "parse_kp_raw")]
    ParseKpRaw {
        kp_base64: String,
    },
    #[serde(rename = "dump_kp_bytes")]
    DumpKpBytes {
        relay: String,
    },
}

#[derive(Serialize)]
struct Response {
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    event_json: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    rumor_json: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pubkey: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    content: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    kind: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    mls_group_id_hex: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    nostr_group_id_hex: Option<String>,
}

impl Response {
    fn ok() -> Self {
        Response {
            ok: true,
            error: None,
            event_json: None,
            rumor_json: None,
            pubkey: None,
            content: None,
            kind: None,
            mls_group_id_hex: None,
            nostr_group_id_hex: None,
        }
    }
    fn err(msg: String) -> Self {
        Response {
            ok: false,
            error: Some(msg),
            event_json: None,
            rumor_json: None,
            pubkey: None,
            content: None,
            kind: None,
            mls_group_id_hex: None,
            nostr_group_id_hex: None,
        }
    }
}

struct State {
    keys: Keys,
    mdk: MDK<MdkMemoryStorage>,
}

fn main() {
    let keys = Keys::generate();
    let mdk = MDK::new(MdkMemoryStorage::default());
    let mut state = State { keys, mdk };

    // Print our pubkey on startup so Go knows it
    let init = serde_json::json!({
        "ready": true,
        "pubkey": state.keys.public_key().to_hex(),
    });
    println!("{}", init);
    io::stdout().flush().unwrap();

    let stdin = io::stdin();
    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };
        if line.trim().is_empty() {
            continue;
        }

        let cmd: Command = match serde_json::from_str(&line) {
            Ok(c) => c,
            Err(e) => {
                let r = Response::err(format!("parse command: {}", e));
                println!("{}", serde_json::to_string(&r).unwrap());
                io::stdout().flush().unwrap();
                continue;
            }
        };

        let resp = handle_command(&mut state, cmd);
        println!("{}", serde_json::to_string(&resp).unwrap());
        io::stdout().flush().unwrap();
    }
}

fn handle_command(state: &mut State, cmd: Command) -> Response {
    match cmd {
        Command::GenerateKeyPackage { relay } => generate_key_package(state, &relay),
        Command::ProcessKeyPackage { event_json } => process_key_package(state, &event_json),
        Command::CreateGroup {
            member_kp_event_json,
            name,
            relay,
        } => create_group(state, &member_kp_event_json, &name, &relay),
        Command::ProcessWelcome { rumor_json } => process_welcome(state, &rumor_json),
        Command::CreateMessage {
            mls_group_id_hex,
            content,
        } => create_message(state, &mls_group_id_hex, &content),
        Command::ProcessMessage { event_json } => process_message(state, &event_json),
        Command::GetGroupInfo {} => get_group_info(state),
        Command::ParseKpRaw { kp_base64 } => parse_kp_raw(&kp_base64),
        Command::DumpKpBytes { relay } => dump_kp_bytes(state, &relay),
    }
}

fn generate_key_package(state: &mut State, relay: &str) -> Response {
    let relay_url = match RelayUrl::parse(relay) {
        Ok(u) => u,
        Err(e) => return Response::err(format!("parse relay URL: {}", e)),
    };

    let (content, tags, _hash_ref) = match state
        .mdk
        .create_key_package_for_event(&state.keys.public_key(), [relay_url])
    {
        Ok(r) => r,
        Err(e) => return Response::err(format!("create key package: {}", e)),
    };

    // Build the kind 443 event
    let event = match EventBuilder::new(Kind::MlsKeyPackage, &content)
        .tags(tags)
        .sign_with_keys(&state.keys)
    {
        Ok(ev) => ev,
        Err(e) => return Response::err(format!("sign key package event: {}", e)),
    };

    let mut resp = Response::ok();
    resp.event_json = Some(event.as_json());
    resp.pubkey = Some(state.keys.public_key().to_hex());
    resp
}

fn process_key_package(_state: &mut State, event_json: &str) -> Response {
    // Just validate we can parse it as a nostr event with the right structure
    let event: Event = match Event::from_json(event_json) {
        Ok(ev) => ev,
        Err(e) => return Response::err(format!("parse event JSON: {}", e)),
    };

    if event.kind != Kind::MlsKeyPackage {
        return Response::err(format!(
            "wrong kind: expected 443, got {}",
            event.kind.as_u16()
        ));
    }

    // Validate content is valid base64
    let content_str = event.content.to_string();
    match BASE64.decode(&content_str) {
        Ok(bytes) => {
            if bytes.len() < 10 {
                return Response::err(format!("key package too short: {} bytes", bytes.len()));
            }
        }
        Err(e) => return Response::err(format!("invalid base64 content: {}", e)),
    }

    // Check required tags
    let has_encoding = event
        .tags
        .iter()
        .any(|t| t.kind().to_string() == "encoding");
    if !has_encoding {
        return Response::err("missing 'encoding' tag".to_string());
    }

    let mut resp = Response::ok();
    resp.pubkey = Some(event.pubkey.to_hex());
    resp
}

fn create_group(
    state: &mut State,
    member_kp_event_json: &str,
    name: &str,
    relay: &str,
) -> Response {
    let member_event: Event = match Event::from_json(member_kp_event_json) {
        Ok(ev) => ev,
        Err(e) => return Response::err(format!("parse member KP event: {}", e)),
    };

    eprintln!("[interop] parse_key_package...");
    match state.mdk.parse_key_package(&member_event) {
        Ok(kp) => eprintln!("[interop] parse_key_package OK, ciphersuite={:?}", kp.ciphersuite()),
        Err(e) => eprintln!("[interop] parse_key_package failed: {}", e),
    }

    let relay_url = match RelayUrl::parse(relay) {
        Ok(u) => u,
        Err(e) => return Response::err(format!("parse relay URL: {}", e)),
    };

    let config = NostrGroupConfigData::new(
        name.to_string(),
        String::new(),
        None,
        None,
        None,
        vec![relay_url],
        vec![state.keys.public_key(), member_event.pubkey],
    );

    eprintln!("[interop] calling create_group...");
    let result = match state.mdk.create_group(
        &state.keys.public_key(),
        vec![member_event],
        config,
    ) {
        Ok(r) => r,
        Err(e) => return Response::err(format!("create group: {}", e)),
    };

    // NOTE: MDK's create_group already calls merge_pending_commit internally
    eprintln!("[interop] create_group succeeded");

    let welcome_rumor = match result.welcome_rumors.first() {
        Some(r) => r,
        None => return Response::err("no welcome rumor generated".to_string()),
    };

    let mut resp = Response::ok();
    resp.rumor_json = Some(welcome_rumor.as_json());
    resp.mls_group_id_hex = Some(hex::encode(result.group.mls_group_id.as_slice()));
    resp.nostr_group_id_hex = Some(hex::encode(&result.group.nostr_group_id));
    resp
}

fn process_welcome(state: &mut State, rumor_json: &str) -> Response {
    let rumor: UnsignedEvent = match UnsignedEvent::from_json(rumor_json) {
        Ok(r) => r,
        Err(e) => return Response::err(format!("parse rumor JSON: {}", e)),
    };

    let welcome = match state
        .mdk
        .process_welcome(&EventId::all_zeros(), &rumor)
    {
        Ok(w) => w,
        Err(e) => return Response::err(format!("process welcome: {}", e)),
    };

    if let Err(e) = state.mdk.accept_welcome(&welcome) {
        return Response::err(format!("accept welcome: {}", e));
    }

    let groups = match state.mdk.get_groups() {
        Ok(g) => g,
        Err(e) => return Response::err(format!("get groups: {}", e)),
    };

    let group = match groups.first() {
        Some(g) => g,
        None => return Response::err("no groups after accepting welcome".to_string()),
    };

    let mut resp = Response::ok();
    resp.mls_group_id_hex = Some(hex::encode(group.mls_group_id.as_slice()));
    resp.nostr_group_id_hex = Some(hex::encode(&group.nostr_group_id));
    resp
}

fn create_message(state: &mut State, mls_group_id_hex: &str, content: &str) -> Response {
    let mls_group_id_bytes = match hex::decode(mls_group_id_hex) {
        Ok(b) => b,
        Err(e) => return Response::err(format!("decode mls_group_id: {}", e)),
    };
    let group_id = GroupId::from_slice(&mls_group_id_bytes);

    let rumor = EventBuilder::new(Kind::Custom(9), content).build(state.keys.public_key());

    let event = match state.mdk.create_message(&group_id, rumor) {
        Ok(ev) => ev,
        Err(e) => return Response::err(format!("create message: {}", e)),
    };

    let mut resp = Response::ok();
    resp.event_json = Some(event.as_json());
    resp
}

fn process_message(state: &mut State, event_json: &str) -> Response {
    let event: Event = match Event::from_json(event_json) {
        Ok(ev) => ev,
        Err(e) => return Response::err(format!("parse event JSON: {}", e)),
    };

    match state.mdk.process_message(&event) {
        Ok(MessageProcessingResult::ApplicationMessage(msg)) => {
            let mut resp = Response::ok();
            resp.content = Some(msg.content.clone());
            resp.kind = Some(msg.kind.as_u16());
            resp.pubkey = Some(msg.pubkey.to_hex());
            resp
        }
        Ok(other) => Response::err(format!("unexpected result: {:?}", other)),
        Err(e) => Response::err(format!("process message: {}", e)),
    }
}

fn get_group_info(state: &mut State) -> Response {
    let groups = match state.mdk.get_groups() {
        Ok(g) => g,
        Err(e) => return Response::err(format!("get groups: {}", e)),
    };

    if groups.is_empty() {
        return Response::err("no groups".to_string());
    }

    let group = &groups[0];
    let mut resp = Response::ok();
    resp.mls_group_id_hex = Some(hex::encode(group.mls_group_id.as_slice()));
    resp.nostr_group_id_hex = Some(hex::encode(&group.nostr_group_id));
    resp
}

fn dump_kp_bytes(state: &mut State, relay: &str) -> Response {
    let relay_url = match RelayUrl::parse(relay) {
        Ok(u) => u,
        Err(e) => return Response::err(format!("parse relay URL: {}", e)),
    };

    let (content_b64, _tags, _hash_ref) = match state
        .mdk
        .create_key_package_for_event(&state.keys.public_key(), [relay_url])
    {
        Ok(r) => r,
        Err(e) => return Response::err(format!("create key package: {}", e)),
    };

    // content_b64 is base64(tls_serialize_detached(KeyPackage))
    let raw_bytes = match BASE64.decode(&content_b64) {
        Ok(b) => b,
        Err(e) => return Response::err(format!("decode own b64: {}", e)),
    };

    let hex_str = hex::encode(&raw_bytes);
    eprintln!("[interop] Rust raw KP bytes ({} bytes): {}", raw_bytes.len(), &hex_str);

    let mut resp = Response::ok();
    resp.content = Some(hex_str);
    resp
}

fn parse_kp_raw(kp_base64: &str) -> Response {
    let kp_bytes = match BASE64.decode(kp_base64) {
        Ok(b) => b,
        Err(e) => return Response::err(format!("base64 decode: {}", e)),
    };

    eprintln!("[interop] raw KP bytes ({} bytes): {}", kp_bytes.len(), hex::encode(&kp_bytes));

    match KeyPackageIn::tls_deserialize(&mut kp_bytes.as_slice()) {
        Ok(_kp_in) => {
            eprintln!("[interop] OpenMLS parsed KeyPackageIn OK");
            let mut resp = Response::ok();
            resp.content = Some(format!("parsed OK"));
            resp
        }
        Err(e) => {
            eprintln!("[interop] OpenMLS deserialize failed: {:?}", e);
            Response::err(format!("tls_deserialize: {:?}", e))
        }
    }
}
