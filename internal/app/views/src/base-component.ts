import { applyComponentStyles } from "./styles.js";

// BaseComponent is the shared foundation for every Project Status Web Component.
// It owns the shadow root and the DOM-building helpers. The helpers are the
// injection-safety seam: every text node is written with textContent and every
// class with the class attribute, so a payload string can never be parsed as
// markup. No helper here (or in any subclass) ever assigns innerHTML from
// payload data — static structure only, populated through textContent/attributes.
export abstract class BaseComponent extends HTMLElement {
  protected readonly view: ShadowRoot;

  constructor() {
    super();
    this.view = this.attachShadow({ mode: "open" });
    applyComponentStyles(this.view);
  }

  // el creates an element, optionally assigning a class attribute and a text
  // node. text is always set via textContent — never innerHTML — so untrusted
  // payload strings render inert.
  protected el(tag: string, className?: string, text?: string): HTMLElement {
    const node = document.createElement(tag);
    if (className !== undefined) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  // badge renders a pill. kind (a payload status/severity/bucket string or a
  // literal like "action-label") becomes an extra CSS class via the class
  // attribute; even a hostile value is inert as a class token, never markup.
  protected badge(text: string, kind?: string): HTMLElement {
    return this.el("span", kind ? `badge ${kind}` : "badge", text);
  }

  // countGrid renders the label/value grid shared by the health, operations, and
  // activity sections. Numbers are stringified through textContent.
  protected countGrid(entries: Array<[string, number]>): HTMLElement {
    const grid = this.el("div", "counts");
    for (const [label, value] of entries) {
      const count = this.el("div", "count");
      count.append(this.el("span", "k", label), this.el("span", "v", String(value)));
      grid.append(count);
    }
    return grid;
  }

  // renderContent replaces the component's rendered output with the given nodes
  // while preserving the adopted/inline styles. Adopted stylesheets are not child
  // nodes, and the inline <style> fallback (if any) is kept.
  protected renderContent(...nodes: Node[]): void {
    const preserved = Array.from(this.view.childNodes).filter(
      (node) => node.nodeName.toLowerCase() === "style",
    );
    this.view.replaceChildren(...preserved, ...nodes);
  }
}
