const { useEffect, useMemo, useState } = React;
const data = window.reasonixRefreshData;

function Icon({ name, size = 15 }) {
  return <i className="icon" data-lucide={name} style={{ width: size, height: size }} aria-hidden="true"></i>;
}

function useLucide() {
  useEffect(() => {
    window.lucide?.createIcons({
      attrs: {
        "stroke-width": 1.8,
      },
    });
  });
}

function TrafficLights() {
  return (
    <div className="traffic" aria-hidden="true">
      <span></span>
      <span></span>
      <span></span>
    </div>
  );
}

function VariantSwitch({ variantId, onVariant, theme, onTheme }) {
  return (
    <div className="switcher" data-screen-label="Variant Switcher">
      <div className="switcher__group" role="tablist" aria-label="Design variants">
        {data.variants.map((variant) => (
          <button
            key={variant.id}
            type="button"
            className={`switcher__item ${variantId === variant.id ? "is-active" : ""}`}
            onClick={() => onVariant(variant.id)}
            role="tab"
            aria-selected={variantId === variant.id}
          >
            <span>{variant.name}</span>
            <em>{variant.short}</em>
          </button>
        ))}
      </div>
      <button className="icon-btn switcher__theme" type="button" onClick={() => onTheme(theme === "dark" ? "light" : "dark")} title="Theme">
        <Icon name={theme === "dark" ? "sun" : "moon"} />
      </button>
    </div>
  );
}

function AppChrome({ activeTab, setActiveTab, openCommand, sidebarOpen, setSidebarOpen, rightOpen, setRightOpen }) {
  return (
    <header className="chrome" data-screen-label="App Chrome">
      <TrafficLights />
      <button className="icon-btn chrome__toggle" type="button" onClick={() => setSidebarOpen(!sidebarOpen)} title="Sidebar">
        <Icon name={sidebarOpen ? "panel-left-close" : "panel-left-open"} />
      </button>
      <div className="tab-strip">
        {data.tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className={`tab ${activeTab === tab.id ? "is-active" : ""}`}
            onClick={() => setActiveTab(tab.id)}
          >
            <span className={`tab__dot ${tab.mode}`}></span>
            <span className="tab__title">{tab.title}</span>
            {tab.mode === "plan" && <span className="tab__mode">plan</span>}
          </button>
        ))}
      </div>
      <button className="command-trigger" type="button" onClick={openCommand}>
        <Icon name="search" />
        <span>Search, command, or open file</span>
        <kbd>⌘K</kbd>
      </button>
      <button className="icon-btn" type="button" title="Workspace" onClick={() => setRightOpen(!rightOpen)}>
        <Icon name={rightOpen ? "panel-right-close" : "panel-right-open"} />
      </button>
    </header>
  );
}

function Sidebar({ sessions, currentSession, setCurrentSession, openCommand }) {
  return (
    <aside className="sidebar" data-screen-label="Sidebar">
      <div className="brand">
        <img src="./logo-symbol.svg" alt="" />
        <div>
          <strong>Reasonix</strong>
          <span>mock model · browser dev</span>
        </div>
      </div>
      <button className="new-session" type="button">
        <Icon name="square-pen" />
        <span>新建会话</span>
      </button>
      <button className="sidebar-search" type="button" onClick={openCommand}>
        <Icon name="search" />
        <span>搜索会话和文件</span>
      </button>
      <div className="section-head">
        <span>会话</span>
        <button type="button">全部</button>
      </div>
      <div className="session-list">
        {sessions.map((session) => (
          <button
            key={session.id}
            type="button"
            className={`session ${currentSession === session.id ? "is-active" : ""}`}
            onClick={() => setCurrentSession(session.id)}
          >
            <span className={`session__mark ${session.tone}`}></span>
            <span className="session__body">
              <strong>{session.title}</strong>
              <small>{session.meta}</small>
            </span>
            {session.unread > 0 && <span className="session__count">{session.unread}</span>}
          </button>
        ))}
      </div>
      <nav className="sidebar-nav">
        <button type="button"><Icon name="brain" />记忆</button>
        <button type="button"><Icon name="blocks" />MCP 与技能</button>
        <button type="button"><Icon name="settings" />设置</button>
      </nav>
    </aside>
  );
}

function TopicBar({ openCommand }) {
  return (
    <div className="topicbar" data-screen-label="Topic Bar">
      <div className="topicbar__title">
        <span className="scope-pill"><Icon name="folder-git-2" />DeepSeek-Reasonix</span>
        <div>
          <strong>重新设计桌面端 UI</strong>
          <span>desktop/frontend · sivancola_20260608_update_ui</span>
        </div>
      </div>
      <div className="topicbar__actions">
        <button className="chip" type="button"><Icon name="git-branch" />3 changed</button>
        <button className="chip" type="button" onClick={openCommand}><Icon name="sparkles" />Command</button>
      </div>
    </div>
  );
}

function UpdateNotice() {
  const [visible, setVisible] = useState(true);
  if (!visible) return null;
  return (
    <div className="notice" data-screen-label="Update Notice">
      <div>
        <strong>发现新版本：v1.1.0</strong>
        <span>更新可在任务结束后安装</span>
      </div>
      <div className="notice__actions">
        <button type="button" className="primary-small">立即更新</button>
        <button type="button" className="ghost-small" onClick={() => setVisible(false)}>稍后</button>
      </div>
    </div>
  );
}

function WelcomePanel({ onPrompt }) {
  return (
    <div className="welcome-panel" data-screen-label="Welcome Panel">
      <img src="./logo.svg" alt="Reasonix" />
      <h1>Reasonix</h1>
      <p>一个编码智能体，描述任务或随便问点什么。</p>
      <div className="hint-row">
        <span><kbd>/</kbd> 命令</span>
        <span><kbd>@</kbd> 引用文件</span>
        <span><kbd>↵</kbd> 发送</span>
      </div>
      <div className="quick-grid">
        {data.quickPrompts.map((prompt) => (
          <button key={prompt} type="button" onClick={() => onPrompt(prompt)}>
            {prompt}
          </button>
        ))}
      </div>
    </div>
  );
}

function ToolSteps({ tools }) {
  const [open, setOpen] = useState("prototype");
  return (
    <div className="tool-stack">
      {tools.map((tool) => (
        <button
          type="button"
          key={tool.name}
          className={`tool-step ${tool.status} ${open === tool.name ? "is-open" : ""}`}
          onClick={() => setOpen(open === tool.name ? "" : tool.name)}
        >
          <span className="tool-step__head">
            <span className="tool-step__status"></span>
            <strong>{tool.name}</strong>
            <small>{tool.status}</small>
            <Icon name="chevron-down" size={13} />
          </span>
          {open === tool.name && (
            <span className="tool-step__body">
              <code>{tool.meta}</code>
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

function Message({ message }) {
  if (message.role === "user") {
    return (
      <article className="message user">
        <div className="message__bubble">{message.text}</div>
      </article>
    );
  }
  return (
    <article className="message assistant">
      {message.reasoning && (
        <details className="reasoning-card" open>
          <summary>
            <Icon name="brain" />
            <span>思考</span>
            <em>done</em>
          </summary>
          <p>{message.reasoning}</p>
        </details>
      )}
      <div className="message__body">
        <p>{message.text}</p>
      </div>
      {message.tools && <ToolSteps tools={message.tools} />}
    </article>
  );
}

function RunRail() {
  return (
    <aside className="run-rail" data-screen-label="Run Rail">
      <div className="run-rail__head">
        <Icon name="activity" />
        <strong>Run</strong>
      </div>
      <ol>
        <li className="done"><span></span>读取 App.tsx</li>
        <li className="done"><span></span>扫描 styles.css</li>
        <li className="active"><span></span>生成 UI 原型</li>
        <li><span></span>等待方案选择</li>
      </ol>
    </aside>
  );
}

function TranscriptArea({ messages, empty, onPrompt }) {
  return (
    <main className={`transcript ${empty ? "is-empty" : ""}`} data-screen-label="Transcript">
      <RunRail />
      {empty ? (
        <WelcomePanel onPrompt={onPrompt} />
      ) : (
        <div className="messages">
          {messages.map((message) => <Message key={message.id} message={message} />)}
        </div>
      )}
    </main>
  );
}

function Composer({ onSend, running }) {
  const [text, setText] = useState("");
  const [mode, setMode] = useState("normal");
  const [shell, setShell] = useState(false);

  const submit = () => {
    const value = text.trim();
    if (!value) return;
    onSend(value);
    setText("");
  };

  return (
    <footer className="footer" data-screen-label="Composer">
      <div className={`composer-card ${shell ? "is-shell" : ""}`}>
        <div className="composer-row">
          <span className="composer-caret">{shell ? "›" : ""}</span>
          <textarea
            value={text}
            onChange={(event) => setText(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                submit();
              }
            }}
            placeholder="给 Reasonix 发消息...（/ 命令 · @ 文件）"
            rows={2}
          ></textarea>
          <button className="send-btn" type="button" onClick={submit} disabled={!text.trim() || running} title="Send">
            <Icon name={running ? "square" : "arrow-up"} />
          </button>
        </div>
        <div className="composer-meta">
          <div className="modebar" role="group" aria-label="Mode">
            {["normal", "plan", "yolo"].map((item) => (
              <button
                key={item}
                type="button"
                className={mode === item ? "is-active" : ""}
                onClick={() => setMode(item)}
              >
                {item}
              </button>
            ))}
          </div>
          <button type="button" className="meta-chip"><Icon name="folder-git-2" />reasonix</button>
          <button type="button" className="meta-chip"><Icon name="brain" />mock model</button>
          <button type="button" className={`meta-chip ${shell ? "is-active" : ""}`} onClick={() => setShell(!shell)}>
            <Icon name="terminal" />shell
          </button>
        </div>
      </div>
    </footer>
  );
}

function WorkspacePanel({ rightPanel, setRightPanel, close }) {
  return (
    <aside className="workspace" data-screen-label="Workspace Panel">
      <div className="workspace__head">
        <div>
          <strong>Workspace</strong>
          <span>desktop/frontend</span>
        </div>
        <button className="icon-btn" type="button" onClick={close} title="Close workspace">
          <Icon name="x" />
        </button>
      </div>
      <div className="workspace-tabs">
        {["files", "changes", "context"].map((tab) => (
          <button
            key={tab}
            type="button"
            className={rightPanel === tab ? "is-active" : ""}
            onClick={() => setRightPanel(tab)}
          >
            {tab}
          </button>
        ))}
      </div>
      {rightPanel === "files" && (
        <div className="file-panel">
          <label className="filter">
            <Icon name="search" />
            <input placeholder="筛选文件..." />
          </label>
          <div className="tree">
            {data.files.map((file) => (
              <button key={`${file.depth}-${file.name}`} type="button" className={file.active ? "is-active" : ""} style={{ "--depth": file.depth }}>
                <Icon name={file.type === "folder" ? (file.open ? "folder-open" : "folder") : "file-text"} />
                <span>{file.name}</span>
                {file.badge && <em>{file.badge}</em>}
              </button>
            ))}
          </div>
        </div>
      )}
      {rightPanel === "changes" && (
        <div className="changes">
          {data.changes.map((change) => (
            <button type="button" key={change.file}>
              <span className={`change-dot ${change.status}`}></span>
              <strong>{change.file}</strong>
              <small>{change.lines}</small>
            </button>
          ))}
        </div>
      )}
      {rightPanel === "context" && (
        <div className="context-panel">
          <section>
            <span className="metric">99.12%</span>
            <small>latest cache hit</small>
          </section>
          <section>
            <span className="metric">0%</span>
            <small>context used</small>
          </section>
          <section>
            <span className="metric">1</span>
            <small>background job</small>
          </section>
        </div>
      )}
    </aside>
  );
}

function CommandPalette({ open, onClose }) {
  if (!open) return null;
  return (
    <div className="palette-layer" data-screen-label="Command Palette" onMouseDown={onClose}>
      <div className="palette" onMouseDown={(event) => event.stopPropagation()}>
        <div className="palette__search">
          <Icon name="search" />
          <input autoFocus placeholder="Search commands, files, sessions..." />
          <kbd>esc</kbd>
        </div>
        <div className="palette__items">
          {data.commandItems.map((item) => (
            <button type="button" key={item.label} onClick={onClose}>
              <Icon name={item.icon} />
              <span>
                <strong>{item.label}</strong>
                <small>{item.meta}</small>
              </span>
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

function StatusBar() {
  return (
    <div className="statusbar" data-screen-label="Status Bar">
      <span className="status-dot"></span>
      <span>mock model</span>
      <span>上下文 {data.status.context}</span>
      <span>缓存 {data.status.cacheNow}</span>
      <span>平均 {data.status.cacheAvg}</span>
      <span>{data.status.cost}</span>
      <span>jobs {data.status.jobs}</span>
      <strong>{data.status.balance}</strong>
    </div>
  );
}

function ReasonixPrototype() {
  const [variantId, setVariantId] = useState("native");
  const [theme, setTheme] = useState("dark");
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [rightOpen, setRightOpen] = useState(true);
  const [rightPanel, setRightPanel] = useState("files");
  const [activeTab, setActiveTab] = useState("t1");
  const [currentSession, setCurrentSession] = useState("s1");
  const [messages, setMessages] = useState(data.initialMessages);
  const [running, setRunning] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  useLucide();

  useEffect(() => {
    const onKey = (event) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommandOpen(true);
      }
      if (event.key === "Escape") setCommandOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const sessions = useMemo(
    () => data.sessions.map((session) => ({ ...session, active: session.id === currentSession })),
    [currentSession],
  );
  const empty = currentSession !== "s1" && messages.length <= data.initialMessages.length;

  const openCommand = () => setCommandOpen(true);
  const sendPrompt = (text) => {
    const userMessage = { id: `u-${Date.now()}`, role: "user", text };
    setMessages((prev) => [...prev, userMessage]);
    setRunning(true);
    window.setTimeout(() => {
      setMessages((prev) => [
        ...prev,
        {
          id: `a-${Date.now()}`,
          role: "assistant",
          text: "收到。我会把这个方向作为下一轮视觉约束，并保持缓存相关实现不动。",
          reasoning: "本次原型只覆盖桌面前端体验，不触碰 provider 请求序列化、系统提示词和 tool schema。",
          tools: [
            { name: "plan", status: "done", meta: "scope: desktop/frontend UI" },
            { name: "render", status: "done", meta: "prototype state updated" },
          ],
        },
      ]);
      setRunning(false);
    }, 650);
  };

  return (
    <div className={`prototype variant-${variantId} theme-${theme} ${sidebarOpen ? "has-sidebar" : "no-sidebar"} ${rightOpen ? "has-workspace" : "no-workspace"}`}>
      <VariantSwitch variantId={variantId} onVariant={setVariantId} theme={theme} onTheme={setTheme} />
      <div className="app-window" data-screen-label="Reasonix Desktop Prototype">
        <AppChrome
          activeTab={activeTab}
          setActiveTab={setActiveTab}
          openCommand={openCommand}
          sidebarOpen={sidebarOpen}
          setSidebarOpen={setSidebarOpen}
          rightOpen={rightOpen}
          setRightOpen={setRightOpen}
        />
        {sidebarOpen && (
          <Sidebar
            sessions={sessions}
            currentSession={currentSession}
            setCurrentSession={setCurrentSession}
            openCommand={openCommand}
          />
        )}
        <section className="chat-pane" data-screen-label="Chat Pane">
          <TopicBar openCommand={openCommand} />
          <UpdateNotice />
          <TranscriptArea messages={messages} empty={empty} onPrompt={sendPrompt} />
          <Composer onSend={sendPrompt} running={running} />
          <StatusBar />
        </section>
        {rightOpen && <WorkspacePanel rightPanel={rightPanel} setRightPanel={setRightPanel} close={() => setRightOpen(false)} />}
      </div>
      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} />
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")).render(<ReasonixPrototype />);
