// Table commands: insert a table, add and remove rows and columns, move
// between cells. Tables are simple (docs/realtime.md#document-model): rows
// of cells holding paragraphs, no merged cells.

import { Fragment, Slice, schema } from "horizon/editor/model.js";
import { Selection } from "horizon/editor/state.js";

const cellOf = (header) => schema.nodes.table_cell.create({ header }, [schema.nodes.paragraph.create()]);

/** A table of rows × cols, its first row of header cells. */
export function createTable(rows = 3, cols = 3) {
  const row = (header) => schema.nodes.table_row.create(null, Array.from({ length: cols }, () => cellOf(header)));
  return schema.nodes.table.create(null, Array.from({ length: rows }, (_, i) => row(i === 0)));
}

/** Where the selection is in a table: depths and indexes, or null. */
export function tableContext(state) {
  const $pos = state.doc.resolve(state.selection.head);
  for (let d = $pos.depth; d > 0; d--) {
    if ($pos.node(d).type.name === "table_cell") {
      return {
        $pos, cellDepth: d, rowDepth: d - 1, tableDepth: d - 2,
        table: $pos.node(d - 2), tablePos: $pos.before(d - 2),
        row: $pos.index(d - 2), col: $pos.index(d - 1),
      };
    }
  }
  return null;
}

/** The position before cell (r, c) of the table at tablePos. */
function cellPos(table, tablePos, r, c) {
  let pos = tablePos + 1;
  for (let i = 0; i < r; i++) pos += table.child(i).nodeSize;
  pos += 1;
  const row = table.child(r);
  for (let j = 0; j < c; j++) pos += row.child(j).nodeSize;
  return pos;
}

/** Moves to the next (dir 1) or previous cell; past the last cell, adds a row. */
export function goToCell(dir) {
  return (state, dispatch) => {
    const ctx = tableContext(state);
    if (!ctx) return false;
    const { table, tablePos } = ctx;
    const cols = table.child(ctx.row).childCount;
    let r = ctx.row;
    let c = ctx.col + dir;
    if (c >= cols) {
      r++;
      c = 0;
    } else if (c < 0) {
      r--;
      c = table.child(Math.max(0, r)).childCount - 1;
    }
    if (r < 0) return false;
    if (!dispatch) return true;
    if (r >= table.childCount) return addRow(1)(state, dispatch);
    const pos = cellPos(table, tablePos, r, c);
    dispatch(state.tr.setSelection(Selection.near(state.doc, pos + 2)));
    return true;
  };
}

/** Adds a row after (dir 1) or before (-1) the current one. */
export function addRow(dir = 1) {
  return (state, dispatch) => {
    const ctx = tableContext(state);
    if (!ctx) return false;
    if (dispatch) {
      const cols = ctx.table.child(ctx.row).childCount;
      const row = schema.nodes.table_row.create(null, Array.from({ length: cols }, () => cellOf(false)));
      const at = dir > 0 ? ctx.$pos.after(ctx.rowDepth) : ctx.$pos.before(ctx.rowDepth);
      const tr = state.tr.insert(at, row);
      dispatch(tr.setSelection(Selection.near(tr.doc, at + 3)));
    }
    return true;
  };
}

/** Adds a column after (dir 1) or before (-1) the current one. */
export function addColumn(dir = 1) {
  return (state, dispatch) => {
    const ctx = tableContext(state);
    if (!ctx) return false;
    if (dispatch) {
      const tr = state.tr;
      const { table, tablePos } = ctx;
      const at = ctx.col + (dir > 0 ? 1 : 0);
      // Last row first, so earlier positions hold.
      for (let r = table.childCount - 1; r >= 0; r--) {
        const row = table.child(r);
        const pos = at < row.childCount ? cellPos(table, tablePos, r, at) : cellPos(table, tablePos, r, row.childCount - 1) + row.lastChild.nodeSize;
        tr.insert(pos, cellOf(r === 0 && row.child(0).attrs.header));
      }
      tr.setSelection(state.selection.map(tr.mapping));
      dispatch(tr);
    }
    return true;
  };
}

/** Removes the current row; the last row takes the table with it. */
export function deleteRow(state, dispatch) {
  const ctx = tableContext(state);
  if (!ctx) return false;
  if (ctx.table.childCount === 1) return deleteTable(state, dispatch);
  if (dispatch) {
    const from = ctx.$pos.before(ctx.rowDepth);
    const to = ctx.$pos.after(ctx.rowDepth);
    const tr = state.tr.delete(from, to);
    dispatch(tr.setSelection(Selection.near(tr.doc, Math.min(from + 3, tr.doc.content.size))));
  }
  return true;
}

/** Removes the current column; the last column takes the table with it. */
export function deleteColumn(state, dispatch) {
  const ctx = tableContext(state);
  if (!ctx) return false;
  if (ctx.table.child(ctx.row).childCount === 1) return deleteTable(state, dispatch);
  if (dispatch) {
    const tr = state.tr;
    const { table, tablePos } = ctx;
    for (let r = table.childCount - 1; r >= 0; r--) {
      if (ctx.col >= table.child(r).childCount) continue;
      const pos = cellPos(table, tablePos, r, ctx.col);
      tr.delete(pos, pos + table.child(r).child(ctx.col).nodeSize);
    }
    dispatch(tr.setSelection(Selection.near(tr.doc, tr.mapping.map(ctx.$pos.pos, -1))));
  }
  return true;
}

/** Removes the whole table, leaving a paragraph. */
export function deleteTable(state, dispatch) {
  const ctx = tableContext(state);
  if (!ctx) return false;
  if (dispatch) {
    const from = ctx.tablePos;
    const tr = state.tr.replace(from, from + ctx.table.nodeSize, new Slice(Fragment.from(schema.nodes.paragraph.create()), 0, 0));
    dispatch(tr.setSelection(new Selection(from + 1)));
  }
  return true;
}
