import re
from dataclasses import dataclass, field


@dataclass
class TreeNode:
    title: str
    level: int
    text: str = ""
    children: list["TreeNode"] = field(default_factory=list)


class PageIndexer:
    """Build and query a hierarchical tree index from parsed document markdown."""

    def build_index(self, markdown: str) -> list[TreeNode]:
        """Build a tree index from markdown headings.

        Parses #/##/### headings into a nested tree structure
        representing Chapters -> Articles -> Paragraphs.
        """
        lines = markdown.split("\n")
        root_nodes: list[TreeNode] = []
        stack: list[TreeNode] = []
        current_text_lines: list[str] = []

        def flush_text() -> None:
            if stack and current_text_lines:
                stack[-1].text = "\n".join(current_text_lines).strip()
            current_text_lines.clear()

        for line in lines:
            heading_match = re.match(r"^(#{1,6})\s+(.+)$", line)
            if heading_match:
                flush_text()
                level = len(heading_match.group(1))
                title = heading_match.group(2).strip()
                node = TreeNode(title=title, level=level)

                # Pop stack until we find a parent at a lower level
                while stack and stack[-1].level >= level:
                    stack.pop()

                if stack:
                    stack[-1].children.append(node)
                else:
                    root_nodes.append(node)

                stack.append(node)
            else:
                current_text_lines.append(line)

        flush_text()
        return root_nodes

    def search_tree(self, index: list[TreeNode], query: str) -> TreeNode | None:
        """Search the tree index for a section matching the query.

        Looks for article/section numbers in the query and matches against node titles.
        """
        num_match = re.search(
            r"(?:Article|Art\.?|Section|Req\.?|Recital|Annex)\s*(\d+)",
            query,
            re.IGNORECASE,
        )
        search_num = num_match.group(1) if num_match else None

        def _search(nodes: list[TreeNode]) -> TreeNode | None:
            for node in nodes:
                if search_num:
                    if re.search(r"\b" + re.escape(search_num) + r"\b", node.title):
                        return node
                elif query.lower() in node.title.lower():
                    return node

                result = _search(node.children)
                if result:
                    return result
            return None

        return _search(index)

    def get_section_text(self, index: list[TreeNode], article_number: str) -> str | None:
        """Get the full text of a specific article by number."""
        result = self.search_tree(index, f"Article {article_number}")
        if result:
            return result.text
        return None
