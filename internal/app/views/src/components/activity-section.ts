import { BaseComponent } from "../base-component.js";
import type { ProjectActivity, ProjectActivityItem } from "../types.js";

// <activity-section> renders a projects.activity payload: the six-count grid and
// the active/blocked/completed/other buckets, each a titled list of work items.
export class ActivitySection extends BaseComponent {
  private data: ProjectActivity | null = null;

  set activity(value: ProjectActivity | null) {
    this.data = value;
    this.render();
  }

  get activity(): ProjectActivity | null {
    return this.data;
  }

  connectedCallback(): void {
    this.render();
  }

  private render(): void {
    const activity = this.data;
    if (!activity) {
      this.renderContent();
      return;
    }

    const section = this.el("section");
    section.append(this.el("h2", undefined, "Activity"));

    const card = this.el("div", "card");
    const counts = activity.counts;
    card.append(
      this.countGrid([
        ["Assignments", counts.assignments],
        ["Active", counts.active],
        ["Blocked", counts.blocked],
        ["Completed", counts.completed],
        ["Awaiting approval", counts.awaiting_approval],
        ["Awaiting review", counts.awaiting_review],
      ]),
    );

    const buckets: Array<[string, ProjectActivityItem[] | undefined]> = [
      ["Active", activity.buckets.active],
      ["Blocked", activity.buckets.blocked],
      ["Completed", activity.buckets.completed],
      ["Other", activity.buckets.other],
    ];
    for (const [label, items] of buckets) {
      if (!items || items.length === 0) continue;
      const bucket = this.el("div", "bucket");
      bucket.append(this.el("div", "bhead", `${label} (${items.length})`));
      const list = this.el("ul");
      for (const item of items) {
        const li = this.el("li", "item");
        const row = this.el("div", "row");
        row.append(
          this.badge(item.status, item.bucket),
          this.el("span", "title", item.work_item_title || item.work_item_id),
        );
        if (item.role_name) row.append(this.el("span", "detail", item.role_name));
        li.append(row);
        list.append(li);
      }
      bucket.append(list);
      card.append(bucket);
    }

    section.append(card);
    this.renderContent(section);
  }
}
