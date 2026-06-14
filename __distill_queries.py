import sqlite3, json, datetime

DB_PATH = r'C:\Users\BoHuYeShan\.local\share\mimocode\mimocode.db'
conn = sqlite3.connect(DB_PATH)
cur = conn.cursor()

# Get recent sessions with their full data
cur.execute("""
    SELECT id, time_created, directory, title, project_id
    FROM session
    WHERE time_created > 1747027200000
    ORDER BY time_created DESC
""")
sessions = cur.fetchall()

print("=== DETAILED SESSION ANALYSIS ===\n")

for s in sessions:
    sid = s[0]
    ts = datetime.datetime.fromtimestamp(s[1]/1000)
    print(f"\n--- Session: {sid} ---")
    print(f"  Time: {ts}")
    print(f"  Dir: {s[2]}")
    print(f"  Title: {s[3][:150]}")
    
    # Get user messages
    cur.execute("""
        SELECT id, json_extract(data, '$.content') as content
        FROM message
        WHERE session_id = ?
          AND json_extract(data, '$.role') = 'user'
        ORDER BY time_created
    """, (sid,))
    user_msgs = cur.fetchall()
    print(f"  User messages: {len(user_msgs)}")
    
    # Get assistant tool usage
    cur.execute("""
        SELECT json_extract(p.data, '$.tool') as tool,
               substr(json_extract(p.data, '$.state.input'), 1, 150) as input_preview,
               count(*) as n
        FROM message m
        JOIN part p ON p.message_id = m.id
        WHERE m.session_id = ?
          AND json_extract(m.data, '$.role') = 'assistant'
          AND json_extract(p.data, '$.type') = 'tool'
        GROUP BY tool, input_preview
        ORDER BY n DESC
        LIMIT 15
    """, (sid,))
    tools = cur.fetchall()
    if tools:
        print("  Top tool patterns:")
        for t in tools:
            print(f"    [{t[2]}x] {t[0]}: {t[1][:120]}")

# Look for repeated command sequences across sessions
print("\n\n=== REPEATED BASH COMMANDS ACROSS ALL SESSIONS ===")
cur.execute("""
    SELECT json_extract(p.data, '$.state.input') as cmd,
           count(*) as n
    FROM message m
    JOIN part p ON p.message_id = m.id
    WHERE json_extract(m.data, '$.role') = 'assistant'
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') = 'Bash'
      AND m.time_created > 1747027200000
    GROUP BY cmd
    ORDER BY n DESC
    LIMIT 30
""")
for row in cur.fetchall():
    print(f"  [{row[1]}x] {row[0][:200]}")

# Look for repeated file edit patterns
print("\n\n=== REPEATED FILE EDIT TARGETS ===")
cur.execute("""
    SELECT json_extract(p.data, '$.state.input') as input,
           count(*) as n
    FROM message m
    JOIN part p ON p.message_id = m.id
    WHERE json_extract(m.data, '$.role') = 'assistant'
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') = 'Edit'
      AND m.time_created > 1747027200000
    GROUP BY json_extract(p.data, '$.state.input')
    ORDER BY n DESC
    LIMIT 20
""")
for row in cur.fetchall():
    # Extract file_path from JSON input
    try:
        inp = json.loads(row[0])
        fp = inp.get('file_path', 'N/A')
        print(f"  [{row[1]}x] {fp}")
    except:
        print(f"  [{row[1]}x] (parse error)")

# Look for Agent/subagent usage patterns
print("\n\n=== AGENT/SUBAGENT USAGE ===")
cur.execute("""
    SELECT m.session_id,
           json_extract(p.data, '$.tool') as tool,
           substr(json_extract(p.data, '$.state.input'), 1, 300) as input_preview
    FROM message m
    JOIN part p ON p.message_id = m.id
    WHERE json_extract(m.data, '$.role') = 'assistant'
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') = 'Agent'
      AND m.time_created > 1747027200000
    ORDER BY m.time_created
    LIMIT 30
""")
for row in cur.fetchall():
    print(f"  [{row[0]}] {row[2]}")

# Look for WebSearch/WebFetch patterns
print("\n\n=== WEB RESEARCH PATTERNS ===")
cur.execute("""
    SELECT m.session_id,
           json_extract(p.data, '$.tool') as tool,
           substr(json_extract(p.data, '$.state.input'), 1, 200) as input_preview
    FROM message m
    JOIN part p ON p.message_id = m.id
    WHERE json_extract(m.data, '$.role') = 'assistant'
      AND json_extract(p.data, '$.type') = 'tool'
      AND json_extract(p.data, '$.tool') IN ('WebSearch', 'WebFetch')
      AND m.time_created > 1747027200000
    ORDER BY m.time_created
    LIMIT 30
""")
for row in cur.fetchall():
    print(f"  [{row[0]}] {row[1]}: {row[2]}")

conn.close()
