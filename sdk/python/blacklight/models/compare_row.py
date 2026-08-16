from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from ..types import UNSET, Unset
from typing import cast


T = TypeVar("T", bound="CompareRow")


@_attrs_define
class CompareRow:
    technique_id: str
    subtechnique_id: str
    name: str
    baseline_category: str
    baseline_protection: str
    current_category: str
    current_protection: str
    classification: str
    baseline_category_ordinal: int | None | Unset = UNSET
    current_category_ordinal: int | None | Unset = UNSET
    ordinal_delta: int | None | Unset = UNSET

    def to_dict(self) -> dict[str, Any]:
        technique_id = self.technique_id

        subtechnique_id = self.subtechnique_id

        name = self.name

        baseline_category = self.baseline_category

        baseline_protection = self.baseline_protection

        current_category = self.current_category

        current_protection = self.current_protection

        classification = self.classification

        baseline_category_ordinal: int | None | Unset
        if isinstance(self.baseline_category_ordinal, Unset):
            baseline_category_ordinal = UNSET
        else:
            baseline_category_ordinal = self.baseline_category_ordinal

        current_category_ordinal: int | None | Unset
        if isinstance(self.current_category_ordinal, Unset):
            current_category_ordinal = UNSET
        else:
            current_category_ordinal = self.current_category_ordinal

        ordinal_delta: int | None | Unset
        if isinstance(self.ordinal_delta, Unset):
            ordinal_delta = UNSET
        else:
            ordinal_delta = self.ordinal_delta

        field_dict: dict[str, Any] = {}

        field_dict.update(
            {
                "techniqueId": technique_id,
                "subtechniqueId": subtechnique_id,
                "name": name,
                "baselineCategory": baseline_category,
                "baselineProtection": baseline_protection,
                "currentCategory": current_category,
                "currentProtection": current_protection,
                "classification": classification,
            }
        )
        if baseline_category_ordinal is not UNSET:
            field_dict["baselineCategoryOrdinal"] = baseline_category_ordinal
        if current_category_ordinal is not UNSET:
            field_dict["currentCategoryOrdinal"] = current_category_ordinal
        if ordinal_delta is not UNSET:
            field_dict["ordinalDelta"] = ordinal_delta

        return field_dict

    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        technique_id = d.pop("techniqueId")

        subtechnique_id = d.pop("subtechniqueId")

        name = d.pop("name")

        baseline_category = d.pop("baselineCategory")

        baseline_protection = d.pop("baselineProtection")

        current_category = d.pop("currentCategory")

        current_protection = d.pop("currentProtection")

        classification = d.pop("classification")

        def _parse_baseline_category_ordinal(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        baseline_category_ordinal = _parse_baseline_category_ordinal(d.pop("baselineCategoryOrdinal", UNSET))

        def _parse_current_category_ordinal(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        current_category_ordinal = _parse_current_category_ordinal(d.pop("currentCategoryOrdinal", UNSET))

        def _parse_ordinal_delta(data: object) -> int | None | Unset:
            if data is None:
                return data
            if isinstance(data, Unset):
                return data
            return cast(int | None | Unset, data)

        ordinal_delta = _parse_ordinal_delta(d.pop("ordinalDelta", UNSET))

        compare_row = cls(
            technique_id=technique_id,
            subtechnique_id=subtechnique_id,
            name=name,
            baseline_category=baseline_category,
            baseline_protection=baseline_protection,
            current_category=current_category,
            current_protection=current_protection,
            classification=classification,
            baseline_category_ordinal=baseline_category_ordinal,
            current_category_ordinal=current_category_ordinal,
            ordinal_delta=ordinal_delta,
        )

        return compare_row
