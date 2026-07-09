import { App } from "@modelcontextprotocol/ext-apps";
import { BaseComponent } from "../base-component.js";
import {
  isRecord,
  type ProjectActivity,
  type ProjectHealth,
  type ProjectOperationsBrief,
  type Structured,
} from "../types.js";
import type { ActivitySection } from "./activity-section.js";
import type { HealthSection } from "./health-section.js";
import type { OperationsBriefSection } from "./operations-brief-section.js";

// State is keyed on the project the sections describe. When a tool result arrives
// for a different project_id than the one held, every section is cleared before
// the new shape is stored, so results from one project never bleed into another
// project's view.
interface ViewState {
  projectId?: string;
  health?: ProjectHealth;
  operations?: ProjectOperationsBrief;
  activity?: ProjectActivity;
}

// <project-status-app> is the root element. It owns the ext-apps handshake, the
// per-project state machine, and dispatch to the three section components. A
// single view backs projects.health, projects.operations_brief, and
// projects.activity: it detects which shape arrived and renders that section,
// accumulating same-project results into one combined view.
export class ProjectStatusApp extends BaseComponent {
  private readonly state: ViewState = {};
  private readonly container: HTMLElement;
  private readonly healthSection: HealthSection;
  private readonly operationsSection: OperationsBriefSection;
  private readonly activitySection: ActivitySection;
  private hostConnected = false;

  constructor() {
    super();
    // Static shell, built once. The h1 and the sections container are the only
    // persistent structure; section elements are reused and toggled per render.
    const shell = this.el("div", "app");
    shell.append(this.el("h1", undefined, "Project Status"));
    this.container = this.el("main", "sections");
    shell.append(this.container);
    this.renderContent(shell);

    this.healthSection = document.createElement("health-section") as HealthSection;
    this.operationsSection = document.createElement(
      "operations-brief-section",
    ) as OperationsBriefSection;
    this.activitySection = document.createElement("activity-section") as ActivitySection;

    this.render();
  }

  connectedCallback(): void {
    this.connectHost();
  }

  // ingest is the single entry point for a structuredContent payload. The host
  // handshake calls it for each tool result; preview and tests call it directly.
  ingest(structuredContent: unknown): void {
    if (!isRecord(structuredContent)) return;
    this.resetForProject(structuredContent);
    this.classify(structuredContent);
    this.render();
  }

  // reset clears all sections back to the waiting state. The host never needs it
  // (a new project_id resets in resetForProject), but the preview harness uses it
  // to switch cleanly between unrelated fixtures.
  reset(): void {
    this.state.projectId = undefined;
    this.state.health = undefined;
    this.state.operations = undefined;
    this.state.activity = undefined;
    this.render();
  }

  // resetForProject clears accumulated sections when the incoming project_id does
  // not match the project currently rendered. Same-project results merge; a new
  // project replaces the view. Payloads without a project_id keep the current
  // project (nothing to key on) rather than merging blindly.
  private resetForProject(data: Structured): void {
    const projectId = typeof data.project_id === "string" ? data.project_id : undefined;
    if (projectId === undefined) return;
    if (this.state.projectId !== projectId) {
      this.state.projectId = projectId;
      this.state.health = undefined;
      this.state.operations = undefined;
      this.state.activity = undefined;
    }
  }

  // classify detects a shape by a field unique to it: health carries `summary`,
  // activity carries `buckets`, and the operations brief carries a
  // `counts.work_items` counter that neither of the others has.
  private classify(data: Structured): void {
    if (isRecord(data.summary)) {
      this.state.health = data as unknown as ProjectHealth;
    }
    if (isRecord(data.buckets)) {
      this.state.activity = data as unknown as ProjectActivity;
    }
    if (isRecord(data.counts) && "work_items" in (data.counts as Structured)) {
      this.state.operations = data as unknown as ProjectOperationsBrief;
    }
  }

  private render(): void {
    const kids: Node[] = [];
    if (!this.state.health && !this.state.operations && !this.state.activity) {
      kids.push(this.el("p", "empty", "Waiting for project status data…"));
    } else {
      if (this.state.health) {
        this.healthSection.health = this.state.health;
        kids.push(this.healthSection);
      }
      if (this.state.operations) {
        this.operationsSection.operations = this.state.operations;
        kids.push(this.operationsSection);
      }
      if (this.state.activity) {
        this.activitySection.activity = this.state.activity;
        kids.push(this.activitySection);
      }
    }
    this.container.replaceChildren(...kids);
  }

  // connectHost performs the host<->view bridge via the official SDK. new App()
  // sets zod to jitless mode (allowUnsafeEval defaults false), so nothing here
  // needs eval/new Function under the strict CSP. The App drives the
  // ui/initialize -> McpUiInitializeResult -> initialized handshake against
  // window.parent; app.ontoolresult fires for each host tool result, whose
  // CallToolResult carries structuredContent. Wrapped in try/catch so a non-host
  // environment (preview at top level, unit tests) degrades gracefully — the view
  // still renders via direct ingest().
  private connectHost(): void {
    if (this.hostConnected) return;
    this.hostConnected = true;
    try {
      const app = new App(
        { name: "cairnline-project-status", version: "1.0.0" },
        { availableDisplayModes: ["inline", "fullscreen"] },
      );
      // Register the result handler before connecting so no early notification is
      // missed, then complete the handshake.
      app.ontoolresult = (result) => {
        this.ingest(result.structuredContent);
      };
      app.connect().catch((err: unknown) => {
        console.error("[project-status] app.connect failed", err);
      });
    } catch (err: unknown) {
      console.error("[project-status] app init failed", err);
    }
  }
}
