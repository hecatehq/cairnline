import { BaseComponent } from "../base-component.js";
import type { ProjectHealth } from "../types.js";

// <health-section> renders a projects.health structuredContent payload: the
// status badge + title, an optional detail line, the eight-count summary grid,
// and the attention list. action_label hints render as inert badges — this view
// never calls tools back.
export class HealthSection extends BaseComponent {
  private data: ProjectHealth | null = null;

  // health is a JS property (not an attribute): the payload is a structured
  // object, so it is passed by reference and rendered through textContent, never
  // serialized into markup.
  set health(value: ProjectHealth | null) {
    this.data = value;
    this.render();
  }

  get health(): ProjectHealth | null {
    return this.data;
  }

  connectedCallback(): void {
    this.render();
  }

  private render(): void {
    const health = this.data;
    if (!health) {
      this.renderContent();
      return;
    }

    const section = this.el("section");
    section.append(this.el("h2", undefined, "Project health"));

    const card = this.el("div", "card");
    const head = this.el("div", "row");
    head.append(this.badge(health.status, health.status), this.el("span", "title", health.title));
    card.append(head);
    if (health.detail) card.append(this.el("p", "detail", health.detail));

    const summary = health.summary;
    card.append(
      this.countGrid([
        ["Attention", summary.attention_count],
        ["Setup to-do", summary.setup_todo_count],
        ["Active assignments", summary.active_assignment_count],
        ["Blocked assignments", summary.blocked_assignment_count],
        ["Open handoffs", summary.open_handoff_count],
        ["Review follow-ups", summary.review_follow_up_count],
        ["Pending memory", summary.pending_memory_candidate_count],
        ["Skill issues", summary.project_skill_issue_count],
      ]),
    );

    const attention = health.attention ?? [];
    if (attention.length > 0) {
      const list = this.el("ul");
      for (const item of attention) {
        const li = this.el("li", "item");
        const row = this.el("div", "row");
        row.append(this.badge(item.severity, item.severity), this.el("span", "title", item.title));
        // Inert affordance: action_label is a plain badge, not a button.
        if (item.action_label) row.append(this.badge(item.action_label, "action-label"));
        li.append(row);
        if (item.detail) li.append(this.el("div", "detail", item.detail));
        list.append(li);
      }
      card.append(list);
    }

    section.append(card);
    this.renderContent(section);
  }
}
