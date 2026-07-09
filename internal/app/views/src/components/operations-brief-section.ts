import { BaseComponent } from "../base-component.js";
import type { ProjectOperationsBrief } from "../types.js";

// <operations-brief-section> renders a projects.operations_brief payload: the
// status badge + title, an optional detail line, the eight-count grid, and the
// operations item list.
export class OperationsBriefSection extends BaseComponent {
  private data: ProjectOperationsBrief | null = null;

  set operations(value: ProjectOperationsBrief | null) {
    this.data = value;
    this.render();
  }

  get operations(): ProjectOperationsBrief | null {
    return this.data;
  }

  connectedCallback(): void {
    this.render();
  }

  private render(): void {
    const brief = this.data;
    if (!brief) {
      this.renderContent();
      return;
    }

    const section = this.el("section");
    section.append(this.el("h2", undefined, "Operations brief"));

    const card = this.el("div", "card");
    const head = this.el("div", "row");
    head.append(this.badge(brief.status, brief.status), this.el("span", "title", brief.title));
    card.append(head);
    if (brief.detail) card.append(this.el("p", "detail", brief.detail));

    const counts = brief.counts;
    card.append(
      this.countGrid([
        ["Open work items", counts.open_work_items],
        ["Active assignments", counts.active_assignments],
        ["Blocked assignments", counts.blocked_assignments],
        ["Review follow-ups", counts.review_follow_ups],
        ["Missing evidence", counts.missing_evidence],
        ["Open handoffs", counts.open_handoffs],
        ["Pending memory", counts.pending_memory_candidates],
        ["Closeout ready", counts.closeout_ready],
      ]),
    );

    const items = brief.items ?? [];
    if (items.length > 0) {
      const list = this.el("ul");
      for (const item of items) {
        const li = this.el("li", "item");
        const row = this.el("div", "row");
        row.append(this.badge(item.severity, item.severity), this.el("span", "title", item.title));
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
