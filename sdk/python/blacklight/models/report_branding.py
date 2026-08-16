from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="ReportBranding")


@_attrs_define
class ReportBranding:
    """Install-wide default branding for report generation. Every field has a
    built-in fallback so a fresh deployment produces readable output without
    configuration. Per-report overrides (client name, logo, colours) take
    precedence over these defaults; the resolution order is defined in
    docs/reporting.md.

    """

    firm_name: str
    """ The firm or team name displayed in report headers and footers. """
    primary_color: str
    """ Primary brand colour as a hex triplet. """
    secondary_color: str
    """ Secondary brand colour as a hex triplet. """
    logo_blob_ref: None | str | Unset = UNSET
    """ Content-addressed blob reference for the logo image. Null means no
    logo is configured. Set this via POST /settings/report-branding/logo
    which returns the blob reference.
     """

    def to_dict(self) -> dict[str, Any]:
        firm_name = self.firm_name

        primary_color = self.primary_color

        secondary_color = self.secondary_color

        logo_blob_ref: None | str | Unset
        if isinstance(self.logo_blob_ref, Unset):
            logo_blob_ref = UNSET
        else:
            logo_blob_ref = self.logo_blob_ref

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "firmName": firm_name,
                "primaryColor": primary_color,
                "secondaryColor": secondary_color,
            }
        )
        if logo_blob_ref is not UNSET:
            field_dict["logoBlobRef"] = logo_blob_ref

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        firm_name = d.pop("firmName")

        primary_color = d.pop("primaryColor")

        secondary_color = d.pop("secondaryColor")

        def _parse_logo_blob_ref(data: object) -> None | str | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(None | str | Unset, data)

        logo_blob_ref = _parse_logo_blob_ref(d.pop("logoBlobRef", UNSET))

        report_branding = cls(
            firm_name=firm_name,
            primary_color=primary_color,
            secondary_color=secondary_color,
            logo_blob_ref=logo_blob_ref,
        )

        return report_branding
