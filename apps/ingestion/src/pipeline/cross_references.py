import re

from src.pipeline.tree_indexer import PageIndexer, TreeNode


class CrossReferenceResolver:
    """Detect and resolve cross-references in regulatory text."""

    # Patterns like "Article 6", "Art. 6(1)", "Recital 47", "Section 3", "Annex II"
    REF_PATTERN = re.compile(
        r"(?:Article|Art\.?|Recital|Section|Annex)\s+(\d+)(?:\((\d+)\))?",
        re.IGNORECASE,
    )

    def __init__(self, indexer: PageIndexer) -> None:
        self.indexer = indexer

    def resolve(
        self, source_text: str, regulation: str, tree_index: list[TreeNode]
    ) -> list[str]:
        """Find cross-references in source_text and resolve them against the tree index.

        Returns a list of reference labels like "GDPR Art. 6(1)".
        """
        matches = self.REF_PATTERN.findall(source_text)
        refs: list[str] = []
        seen: set[str] = set()

        for article_num, paragraph in matches:
            # Build a reference label
            label = f"{regulation} Art. {article_num}"
            if paragraph:
                label += f"({paragraph})"

            if label in seen:
                continue
            seen.add(label)

            # Verify the reference exists in the document tree
            node = self.indexer.search_tree(tree_index, f"Article {article_num}")
            if node:
                refs.append(label)

        return refs
