from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field
import json
from .. import types

from ..types import UNSET, Unset

from ..types import File, FileTypes
from ..types import UNSET, Unset
from io import BytesIO


T = TypeVar("T", bound="UploadContentBundleRequest")


@_attrs_define
class UploadContentBundleRequest:
    """Multipart offline bundle upload. `file` is the release archive; optional
    `version` pins ATT&CK multi-version sources the same way sync does.

    """

    file: File
    """ Release archive bytes (typically `.zip` or `.tar.gz`) matching the
    adapter's online fetch shape. Size is capped by
    `BLACKLIGHT_CONTENT_MAX_BYTES` (default 512 MiB).
     """
    version: str | Unset = UNSET
    """ ATT&CK release label (e.g. `15.1`). Omit for latest discoverable /
    the version embedded in the archive. Ignored by rolling sources.
     """

    def to_dict(self) -> dict[str, Any]:
        file = self.file.to_tuple()

        version = self.version

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "file": file,
            }
        )
        if version is not UNSET:
            field_dict["version"] = version

        return field_dict

    def to_multipart(self) -> types.RequestFiles:
        files: types.RequestFiles = []

        files.append(("file", self.file.to_tuple()))

        if not isinstance(self.version, Unset):
            files.append(("version", (None, str(self.version).encode(), "text/plain")))

        return files

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        file = File(payload=BytesIO(d.pop("file")))

        version = d.pop("version", UNSET)

        upload_content_bundle_request = cls(
            file=file,
            version=version,
        )

        return upload_content_bundle_request
