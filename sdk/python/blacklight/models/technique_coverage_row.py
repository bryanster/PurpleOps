from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="TechniqueCoverageRow")


@_attrs_define
class TechniqueCoverageRow:
    technique_id: str
    name: str
    is_subtechnique: bool
    parent_technique_id: str
    matched: bool
    attempted: bool
    best_category: str
    best_protection: str
    step_count: int
    best_category_ordinal: int | None | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        technique_id = self.technique_id

        name = self.name

        is_subtechnique = self.is_subtechnique

        parent_technique_id = self.parent_technique_id

        matched = self.matched

        attempted = self.attempted

        best_category = self.best_category

        best_protection = self.best_protection

        step_count = self.step_count

        best_category_ordinal: int | None | Unset
        if isinstance(self.best_category_ordinal, Unset):
            best_category_ordinal = UNSET
        else:
            best_category_ordinal = self.best_category_ordinal

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "techniqueId": technique_id,
                "name": name,
                "isSubtechnique": is_subtechnique,
                "parentTechniqueId": parent_technique_id,
                "matched": matched,
                "attempted": attempted,
                "bestCategory": best_category,
                "bestProtection": best_protection,
                "stepCount": step_count,
            }
        )
        if best_category_ordinal is not UNSET:
            field_dict["bestCategoryOrdinal"] = best_category_ordinal

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        technique_id = d.pop("techniqueId")

        name = d.pop("name")

        is_subtechnique = d.pop("isSubtechnique")

        parent_technique_id = d.pop("parentTechniqueId")

        matched = d.pop("matched")

        attempted = d.pop("attempted")

        best_category = d.pop("bestCategory")

        best_protection = d.pop("bestProtection")

        step_count = d.pop("stepCount")

        def _parse_best_category_ordinal(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        best_category_ordinal = _parse_best_category_ordinal(d.pop("bestCategoryOrdinal", UNSET))

        technique_coverage_row = cls(
            technique_id=technique_id,
            name=name,
            is_subtechnique=is_subtechnique,
            parent_technique_id=parent_technique_id,
            matched=matched,
            attempted=attempted,
            best_category=best_category,
            best_protection=best_protection,
            step_count=step_count,
            best_category_ordinal=best_category_ordinal,
        )

        return technique_coverage_row
