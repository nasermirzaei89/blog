import "@fontsource/open-sans";
import Alpine from 'alpinejs'
import ajax from '@imacrayon/alpine-ajax'
import { Editor } from '@tiptap/core'
import Underline from '@tiptap/extension-underline'
import BulletList from '@tiptap/extension-bullet-list'
import Text from '@tiptap/extension-text'
import Document from '@tiptap/extension-document'
import Blockquote from '@tiptap/extension-blockquote'
import CodeBlock from '@tiptap/extension-code-block'
import HardBreak from '@tiptap/extension-hard-break'
import Heading from '@tiptap/extension-heading'
import HorizontalRule from '@tiptap/extension-horizontal-rule'
import ListItem from '@tiptap/extension-list-item'
import OrderedList from '@tiptap/extension-ordered-list'
import Paragraph from '@tiptap/extension-paragraph'
import Bold from '@tiptap/extension-bold'
import Code from '@tiptap/extension-code'
import Italic from '@tiptap/extension-italic'
import Strike from '@tiptap/extension-strike'
import Dropcursor from '@tiptap/extension-dropcursor'
import Gapcursor from '@tiptap/extension-gapcursor'
import { UndoRedo } from '@tiptap/extensions'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'

const initWysiwygEditor = (element) => {
    const editorContainer = document.createElement("div");
    editorContainer.setAttribute("id", `${element.id}-wysiwyg-editor`);
    editorContainer.classList.add("wysiwyg-editor");

    const editorToolbar = document.createElement("div");
    editorToolbar.setAttribute("id", `${element.id}-wysiwyg-editor-toolbar`);
    editorToolbar.classList.add("wysiwyg-editor-toolbar");
    editorContainer.appendChild(editorToolbar);

    const editorContent = document.createElement("div");
    editorContent.setAttribute("id", `${element.id}-wysiwyg-editor-content`);
    editorContainer.appendChild(editorContent);

    element.insertAdjacentElement("afterend", editorContainer);
    element.classList.add("!hidden");

    const editor = new Editor({
        element: editorContent,
        extensions: [
            CodeBlock,
            Document,
            HardBreak,
            HorizontalRule,
            Text,
            Code,

            Dropcursor,
            Gapcursor,
            UndoRedo,

            Heading,
            Paragraph,

            Bold,
            Italic,
            Underline,
            Strike,

            BulletList,
            OrderedList,
            ListItem,

            Blockquote,

            Link.configure({
                openOnClick: false,
            }),
            Image.configure({
                inline: true,
                allowBase64: true,
            }),
        ],
        content: element.value,
        editorProps: {
            attributes: {
                class: "wysiwyg-editor-content"
            },
        },
        onUpdate({ editor }) {
            element.value = editor.getHTML();
        },
    });

    // Add toolbar
    const toolbar = [
        [
            {
                title: "Heading 2",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M3 4h2v6h4V4h2v14H9v-6H5v6H3zm18 14h-6a2 2 0 0 1-2-2c0-.53.2-1 .54-1.36l4.87-5.23c.37-.36.59-.86.59-1.41a2 2 0 0 0-2-2a2 2 0 0 0-2 2h-2a4 4 0 0 1 4-4a4 4 0 0 1 4 4c0 1.1-.45 2.1-1.17 2.83L15 16h6z"/></svg>`,
                onClick: () => editor.chain().focus().toggleHeading({ level: 2 }).run(),
                isActive: (editor) => editor.isActive('heading', { level: 2 })
            },
            {
                title: "Heading 3",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M3 4h2v6h4V4h2v14H9v-6H5v6H3zm12 0h4a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2h-4a2 2 0 0 1-2-2v-1h2v1h4v-4h-4v-2h4V6h-4v1h-2V6a2 2 0 0 1 2-2"/></svg>`,
                onClick: () => editor.chain().focus().toggleHeading({ level: 3 }).run(),
                isActive: (editor) => editor.isActive('heading', { level: 3 })
            },
        ],
        [
            {
                title: "Bold",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M13.5 15.5H10v-3h3.5A1.5 1.5 0 0 1 15 14a1.5 1.5 0 0 1-1.5 1.5m-3.5-9h3A1.5 1.5 0 0 1 14.5 8A1.5 1.5 0 0 1 13 9.5h-3m5.6 1.29c.97-.68 1.65-1.79 1.65-2.79c0-2.26-1.75-4-4-4H7v14h7.04c2.1 0 3.71-1.7 3.71-3.79c0-1.52-.86-2.82-2.15-3.42"/></svg>`,
                onClick: () => editor.chain().focus().toggleBold().run(),
                isActive: (editor) => editor.isActive('bold')
            },
            {
                title: "Italic",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M10 4v3h2.21l-3.42 8H6v3h8v-3h-2.21l3.42-8H18V4z"/></svg>`,
                onClick: () => editor.chain().focus().toggleItalic().run(),
                isActive: (editor) => editor.isActive('italic')
            },
            {
                title: "Underline",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M5 21h14v-2H5zm7-4a6 6 0 0 0 6-6V3h-2.5v8a3.5 3.5 0 0 1-3.5 3.5A3.5 3.5 0 0 1 8.5 11V3H6v8a6 6 0 0 0 6 6"/></svg>`,
                onClick: () => editor.chain().focus().toggleUnderline().run(),
                isActive: (editor) => editor.isActive('underline')
            },
            {
                title: "Strikethrough",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M7.2 9.8c-1.2-2.3.5-5 2.9-5.5c3.1-1 7.6.4 7.5 4.2h-3c0-.3-.1-.6-.1-.8c-.2-.6-.6-.9-1.2-1.1c-.8-.3-2.1-.2-2.8.3C9 8.2 10.4 9.5 12 10H7.4c-.1-.1-.1-.2-.2-.2M21 13v-2H3v2h9.6c.2.1.4.1.6.2c.6.3 1.1.5 1.3 1.1c.1.4.2.9 0 1.3c-.2.5-.6.7-1.1.9c-1.8.5-4-.2-3.9-2.4h-3c-.1 2.6 2.1 4.4 4.5 4.7c3.8.8 8.3-1.6 6.3-5.9z"/></svg>`,
                onClick: () => editor.chain().focus().toggleStrike().run(),
                isActive: (editor) => editor.isActive('strike')
            },
            {
                title: "Code",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="m14.6 16.6l4.6-4.6l-4.6-4.6L16 6l6 6l-6 6zm-5.2 0L4.8 12l4.6-4.6L8 6l-6 6l6 6z"/></svg>`,
                onClick: () => editor.chain().focus().toggleCode().run(),
                isActive: (editor) => editor.isActive('code')
            },
        ],
        [
            {
                title: "Bulleted List",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M7 5h14v2H7zm0 8v-2h14v2zM4 4.5A1.5 1.5 0 0 1 5.5 6A1.5 1.5 0 0 1 4 7.5A1.5 1.5 0 0 1 2.5 6A1.5 1.5 0 0 1 4 4.5m0 6A1.5 1.5 0 0 1 5.5 12A1.5 1.5 0 0 1 4 13.5A1.5 1.5 0 0 1 2.5 12A1.5 1.5 0 0 1 4 10.5M7 19v-2h14v2zm-3-2.5A1.5 1.5 0 0 1 5.5 18A1.5 1.5 0 0 1 4 19.5A1.5 1.5 0 0 1 2.5 18A1.5 1.5 0 0 1 4 16.5"/></svg>`,
                onClick: () => editor.chain().focus().toggleBulletList().run(),
                isActive: (editor) => editor.isActive('bulletList')
            },
            {
                title: "Numbered List",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M7 13v-2h14v2zm0 6v-2h14v2zM7 7V5h14v2zM3 8V5H2V4h2v4zm-1 9v-1h3v4H2v-1h2v-.5H3v-1h1V17zm2.25-7a.75.75 0 0 1 .75.75c0 .2-.08.39-.21.52L3.12 13H5v1H2v-.92L4 11H2v-1z"/></svg>`,
                onClick: () => editor.chain().focus().toggleOrderedList().run(),
                isActive: (editor) => editor.isActive('orderedList')
            },
        ],
        [
            {
                title: "Blockquote",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="m10 7l-2 4h3v6H5v-6l2-4zm8 0l-2 4h3v6h-6v-6l2-4z"/></svg>`,
                onClick: () => editor.chain().focus().toggleBlockquote().run(),
                isActive: (editor) => editor.isActive('blockquote')
            },
        ],
        [
            {
                title: "Link",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7a5 5 0 0 0-5 5a5 5 0 0 0 5 5h4v-1.9H7c-1.71 0-3.1-1.39-3.1-3.1M8 13h8v-2H8zm9-6h-4v1.9h4c1.71 0 3.1 1.39 3.1 3.1s-1.39 3.1-3.1 3.1h-4V17h4a5 5 0 0 0 5-5a5 5 0 0 0-5-5"/></svg>`,
                onClick: () => {
                    const previousUrl = editor.getAttributes('link').href
                    const url = window.prompt('URL', previousUrl || 'https://')
                    // cancelled
                    if (url === null) {
                        return
                    }
                    // empty
                    if (url === '') {
                        editor.chain().focus().extendMarkRange('link').unsetLink().run()
                        return
                    }
                    // update link
                    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
                },
                isActive: (editor) => editor.isActive('link')
            },
            {
                title: "Image",
                content: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24"><path fill="currentColor" d="m8.5 13.5l2.5 3l3.5-4.5l4.5 6H5m16 1V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2"/></svg>`,
                onClick: () => {
                    const url = window.prompt('Image URL', editor.getAttributes('image').src || 'https://')
                    if (url) {
                        editor.chain().focus().setImage({ src: url }).run()
                    }
                },
                isActive: (editor) => editor.isActive('image')
            },
        ],
    ]

    // Create toolbar buttons with active state tracking
    const toolbarButtons = new Map(); // Store button references for active state updates

    toolbar.forEach((toolset) => {
        const div = document.createElement("div");
        div.classList.add("toolset");

        toolset.forEach((tool) => {
            const button = document.createElement("button");
            button.innerHTML = tool.content;
            button.type = "button";
            button.title = tool.title;
            button.onclick = tool.onClick;

            // Store button reference with its check function
            if (tool.isActive) {
                toolbarButtons.set(button, tool.isActive);
            }

            div.appendChild(button);
        });

        editorToolbar.appendChild(div);
    });

    // Function to update button active states
    const updateButtonStates = () => {
        toolbarButtons.forEach((isActiveCheck, button) => {
            if (isActiveCheck(editor)) {
                button.classList.add('active');
            } else {
                button.classList.remove('active');
            }
        });
    };

    // Listen for selection changes to update button states
    editor.on('selectionUpdate', updateButtonStates);
    editor.on('transaction', updateButtonStates);
}

const selector = "textarea[data-wysiwyg-editor]"

const observer = new MutationObserver(mutations => {
    mutations.forEach(mutation => {
        mutation.addedNodes.forEach((node) => {
            if (node.nodeType === 1) { // Ensure it's an element node
                if (node.matches(selector)) {
                    initWysiwygEditor(node);
                } else {
                    // Check inside the node for any matching textarea
                    node.querySelectorAll?.(selector).forEach(initWysiwygEditor);
                }
            }
        });
    });
});

// Observe the whole document for added elements
observer.observe(document.body, { childList: true, subtree: true });

const elements = document.querySelectorAll(selector)
elements.forEach(initWysiwygEditor);

// Initialize Alpine.js with Alpine AJAX plugin
window.Alpine = Alpine
Alpine.plugin(ajax)
Alpine.start()
